package renderwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

func exampleBundle(t *testing.T) Bundle {
	t.Helper()
	hash := sha256.Sum256([]byte("abc"))
	file := storagev1.SnapshotFile{Path: "a.typ", Size: 3, SHA256: hex.EncodeToString(hash[:])}
	m := storagev1.SnapshotManifest{FormatVersion: 1, Files: []storagev1.SnapshotFile{file}, Directories: []string{"empty"}, Fonts: []storagev1.SnapshotFont{{Path: ".", InProject: false, SourcePath: "/removed/fonts", Files: []storagev1.SnapshotFile{file}, Directories: []string{}}}}
	id, err := storagev1.ManifestContentID(m)
	if err != nil {
		t.Fatal(err)
	}
	m.ID = id
	return Bundle{Manifest: m, ProjectFS: fstest.MapFS{"a.typ": {Data: []byte("abc")}}, ExternalFontFS: map[int]fs.FS{0: fstest.MapFS{"a.typ": {Data: []byte("abc")}}}}
}
func TestBundleRoundTrip(t *testing.T) {
	bundle := exampleBundle(t)
	var b bytes.Buffer
	e := NewEncoder(&b, InputLimit)
	if err := WriteBundle(e, bundle); err != nil {
		t.Fatal(err)
	}
	_ = e.WriteBytes(Terminal, []byte(`{}`))
	d := NewDecoder(&b, InputLimit)
	var got []FileDescriptor
	m, err := ReadBundle(d, func(f FileDescriptor, r io.Reader) error {
		data, err := io.ReadAll(r)
		if string(data) != "abc" {
			t.Fatal("changed bytes")
		}
		got = append(got, f)
		return err
	})
	if err != nil || m.ID != bundle.Manifest.ID || len(got) != 2 || got[1].Tree != "font" || got[1].FontIndex != 0 {
		t.Fatal(m.ID, got, err)
	}
	if _, err = d.ReadBytes(Terminal); err != nil {
		t.Fatal(err)
	}
	if err = d.Finish(); err != nil {
		t.Fatal(err)
	}
}
func TestBundleRejectsWrongPathHashSizeAndMetadata(t *testing.T) {
	bundle := exampleBundle(t)
	for _, mutate := range []func(*FileDescriptor){func(f *FileDescriptor) { f.Path = "../a.typ" }, func(f *FileDescriptor) { f.Tree = "font" }, func(f *FileDescriptor) { f.Size = 4 }, func(f *FileDescriptor) { f.SHA256 = string(bytes.Repeat([]byte("a"), 64)) }} {
		var b bytes.Buffer
		e := NewEncoder(&b, InputLimit)
		data, _ := json.Marshal(bundle.Manifest)
		_ = e.WriteBytes(Manifest, data)
		f := FileDescriptor{Tree: "project", Path: "a.typ", Size: 3, SHA256: bundle.Manifest.Files[0].SHA256}
		mutate(&f)
		data, _ = json.Marshal(f)
		_ = e.WriteBytes(FileHeader, data)
		_ = e.WriteBytes(File, []byte("abc"))
		if _, err := ReadBundle(NewDecoder(&b, InputLimit), func(_ FileDescriptor, r io.Reader) error { _, err := io.Copy(io.Discard, r); return err }); err == nil {
			t.Fatal("accepted altered binding", f)
		}
	}
	bundle.Manifest.Files[0].Size = SnapshotLimit + 1
	if err := WriteBundle(NewEncoder(io.Discard, InputLimit), bundle); err == nil {
		t.Fatal("oversize snapshot")
	}
}
func TestBundleRejectsMissingExternalTreeAndCorruptFile(t *testing.T) {
	bundle := exampleBundle(t)
	delete(bundle.ExternalFontFS, 0)
	if err := WriteBundle(NewEncoder(io.Discard, InputLimit), bundle); err == nil {
		t.Fatal("missing font tree")
	}
	bundle = exampleBundle(t)
	bundle.ProjectFS = fstest.MapFS{"a.typ": {Data: []byte("xyz")}}
	if err := WriteBundle(NewEncoder(io.Discard, InputLimit), bundle); err == nil {
		t.Fatal("changed data")
	}
}

func TestBundleRejectsEscapedDescriptorBeforeOpeningFiles(t *testing.T) {
	bundle := exampleBundle(t)
	bundle.Manifest.Files[0].Path = strings.Repeat("\x01", 4096)
	id, err := storagev1.ManifestContentID(bundle.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	bundle.Manifest.ID = id
	if _, err := PlanFiles(bundle.Manifest); err == nil {
		t.Fatal("accepted descriptor larger than its frame limit")
	}
}
