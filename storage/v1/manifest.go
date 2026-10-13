package storagev1

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"strings"
)

func validHash(s string) bool {
	if len(s) != 64 || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func validPath(s string, root bool) bool {
	return fs.ValidPath(s) && !strings.ContainsAny(s, "\\\x00") && (root || s != ".")
}
func validateTree(files []SnapshotFile, dirs []string) error {
	if files == nil || dirs == nil {
		return fmt.Errorf("manifest arrays must be non-null")
	}
	names := map[string]bool{}
	previous := ""
	for _, d := range dirs {
		if !validPath(d, false) || d <= previous {
			return fmt.Errorf("directories must be valid and strictly sorted")
		}
		names[d] = true
		previous = d
	}
	previous = ""
	for _, f := range files {
		if !validPath(f.Path, false) || f.Path <= previous || f.Size < 0 || !validHash(f.SHA256) || names[f.Path] {
			return fmt.Errorf("invalid, duplicate or unsorted file %q", f.Path)
		}
		previous = f.Path
		parent := path.Dir(f.Path)
		if parent != "." && !names[parent] {
			return fmt.Errorf("missing parent of %q", f.Path)
		}
	}
	for _, d := range dirs {
		if parent := path.Dir(d); parent != "." && !names[parent] {
			return fmt.Errorf("missing parent of %q", d)
		}
	}
	return nil
}
func validateManifestShape(m SnapshotManifest) error {
	if m.FormatVersion != 1 || m.Fonts == nil {
		return fmt.Errorf("unsupported or non-canonical snapshot manifest")
	}
	if err := validateTree(m.Files, m.Directories); err != nil {
		return err
	}
	for i, font := range m.Fonts {
		if err := validateTree(font.Files, font.Directories); err != nil {
			return fmt.Errorf("font %d: %w", i, err)
		}
		if !font.InProject {
			if font.Path != "." {
				return fmt.Errorf("external font %d must have logical root", i)
			}
			continue
		}
		if !validPath(font.Path, true) {
			return fmt.Errorf("invalid in-project font path")
		}
		if font.Path == "." {
			if !reflect.DeepEqual(font.Files, m.Files) || !reflect.DeepEqual(font.Directories, m.Directories) {
				return fmt.Errorf("font tree differs from project root")
			}
			continue
		}
		found := false
		for _, d := range m.Directories {
			if d == font.Path {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("font directory missing in project")
		}
		files := []SnapshotFile{}
		dirs := []string{}
		prefix := font.Path + "/"
		for _, f := range m.Files {
			if strings.HasPrefix(f.Path, prefix) {
				f.Path = strings.TrimPrefix(f.Path, prefix)
				files = append(files, f)
			}
		}
		for _, d := range m.Directories {
			if strings.HasPrefix(d, prefix) {
				dirs = append(dirs, strings.TrimPrefix(d, prefix))
			}
		}
		if !reflect.DeepEqual(files, font.Files) || !reflect.DeepEqual(dirs, font.Directories) {
			return fmt.Errorf("font subtree differs from project")
		}
	}
	return nil
}

// ManifestContentID retains the M02 v1 content projection exactly. Wire JSON and provenance are not the identity.
func ManifestContentID(m SnapshotManifest) (string, error) {
	if err := validateManifestShape(m); err != nil {
		return "", err
	}
	type contentFont struct {
		Path        string
		Files       []SnapshotFile
		Directories []string
	}
	content := struct {
		FormatVersion int
		Files         []SnapshotFile
		Directories   []string
		Fonts         []contentFont
	}{m.FormatVersion, m.Files, m.Directories, make([]contentFont, len(m.Fonts))}
	for i, f := range m.Fonts {
		content.Fonts[i] = contentFont{f.Path, f.Files, f.Directories}
	}
	data, err := json.Marshal(content)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
func ValidateManifest(m SnapshotManifest) error {
	id, err := ManifestContentID(m)
	if err != nil {
		return err
	}
	if !validHash(m.ID) || m.ID != id {
		return fmt.Errorf("snapshot content identity mismatch")
	}
	return nil
}
