package renderwire

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
	"io"
	"io/fs"
	"strings"

	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

// Bundle borrows immutable logical trees. Only external font manifest indices
// occur in ExternalFontFS; in-project font trees are already in ProjectFS.
type Bundle struct {
	Manifest       storagev1.SnapshotManifest
	ProjectFS      fs.FS
	ExternalFontFS map[int]fs.FS
}
type FileDescriptor struct {
	Tree      string `json:"tree"`
	FontIndex int    `json:"fontIndex,omitempty"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

// PlanFiles validates manifest identity and bounds the transported bytes, file
// count and combined manifest/descriptor metadata before opening any files.
func PlanFiles(manifest storagev1.SnapshotManifest) ([]FileDescriptor, error) {
	if err := storagev1.ValidateManifest(manifest); err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	metadataSize := int64(len(metadata))
	var files []FileDescriptor
	var total int64
	add := func(tree string, index int, entries []storagev1.SnapshotFile, dirs []string) error {
		for _, dir := range dirs {
			if len(dir) > 4096 {
				return errors.New("directory path exceeds limit")
			}
		}
		for _, file := range entries {
			if len(file.Path) > 4096 || file.Size > SnapshotLimit || total > SnapshotLimit-file.Size {
				return errors.New("snapshot bytes or path limit exceeded")
			}
			total += file.Size
			f := FileDescriptor{Tree: tree, FontIndex: index, Path: file.Path, Size: file.Size, SHA256: file.SHA256}
			files = append(files, f)
			encoded, _ := json.Marshal(f)
			if int64(len(encoded)) > bound(FileHeader) {
				return errors.New("file descriptor exceeds frame limit")
			}
			metadataSize += int64(len(encoded))
			if len(files) > 10000 || metadataSize > MetadataLimit {
				return errors.New("bundle metadata or file count exceeds limit")
			}
		}
		return nil
	}
	if metadataSize > MetadataLimit {
		return nil, errors.New("bundle metadata exceeds limit")
	}
	if err = add("project", 0, manifest.Files, manifest.Directories); err != nil {
		return nil, err
	}
	for index, font := range manifest.Fonts {
		if !font.InProject {
			if err = add("font", index, font.Files, font.Directories); err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

func WriteBundle(e *Encoder, bundle Bundle) error {
	files, err := PlanFiles(bundle.Manifest)
	if err != nil {
		return err
	}
	expectedFonts := 0
	for i, f := range bundle.Manifest.Fonts {
		if !f.InProject {
			expectedFonts++
			if bundle.ExternalFontFS[i] == nil {
				return errors.New("missing captured font tree")
			}
		}
	}
	if len(bundle.ExternalFontFS) != expectedFonts || bundle.ProjectFS == nil {
		return errors.New("bundle trees differ from manifest")
	}
	metadata, _ := json.Marshal(bundle.Manifest)
	if err = e.WriteBytes(Manifest, metadata); err != nil {
		return err
	}
	for _, file := range files {
		tree := bundle.ProjectFS
		if file.Tree == "font" {
			tree = bundle.ExternalFontFS[file.FontIndex]
		}
		if err = writeBundleFile(e, tree, file); err != nil {
			return err
		}
	}
	return nil
}
func writeBundleFile(e *Encoder, tree fs.FS, file FileDescriptor) error {
	f, err := tree.Open(file.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != file.Size {
		return errors.New("bundle file type or size differs from manifest")
	}
	metadata, _ := json.Marshal(file)
	if err = e.WriteBytes(FileHeader, metadata); err != nil {
		return err
	}
	digest, _ := hex.DecodeString(file.SHA256)
	var hash [32]byte
	copy(hash[:], digest)
	return e.WriteFrom(File, file.Size, hash, f)
}

// ReadBundle requires exactly the manifest's ordered files and hashes, passing
// each bounded stream to sink. The caller owns private staging, discards it on
// error, and must validate Terminal, EOF and container exit before publication.
func ReadBundle(d *Decoder, sink func(FileDescriptor, io.Reader) error) (manifest storagev1.SnapshotManifest, err error) {
	data, err := d.ReadBytes(Manifest)
	if err != nil {
		return manifest, err
	}
	if err = DecodeJSON(data, &manifest); err != nil {
		return manifest, err
	}
	files, err := PlanFiles(manifest)
	if err != nil {
		return manifest, err
	}
	metadataSize := int64(len(data))
	for _, expected := range files {
		var file FileDescriptor
		data, err = d.ReadBytes(FileHeader)
		if err != nil {
			return manifest, err
		}
		metadataSize += int64(len(data))
		if metadataSize > MetadataLimit {
			return manifest, errors.New("received bundle metadata exceeds limit")
		}
		if err = DecodeJSON(data, &file); err != nil {
			return manifest, err
		}
		if file != expected {
			return manifest, fmt.Errorf("file descriptor differs from manifest: %s", expected.Path)
		}
		err = d.Read(File, func(h Header, r io.Reader) error {
			if h.Size != expected.Size || hex.EncodeToString(h.SHA256[:]) != expected.SHA256 {
				return errors.New("file frame differs from manifest")
			}
			return sink(expected, r)
		})
		if err != nil {
			return manifest, err
		}
	}
	return manifest, nil
}

// DecodeJSON rejects unknown fields and trailing JSON values. Frame length and
// digest have already been verified by Decoder. This is metadata, not parameter
// normalization; callers retain RawMessage parameter bytes unchanged.
func DecodeJSON(data []byte, out any) error {
	return jsonvalue.Unmarshal(data, out, MetadataLimit)
}

// SafeLogicalPath excludes native separators and control bytes. Native staging
// must additionally avoid following links; opaque staging file names are safer.
func SafeLogicalPath(name string) bool {
	return fs.ValidPath(name) && name != "." && len(name) <= 4096 && !strings.ContainsAny(name, "\\\x00")
}
