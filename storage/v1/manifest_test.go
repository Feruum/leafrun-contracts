package storagev1

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestManifestIdentityPreservesM02EncodingAndIgnoresProvenance(t *testing.T) {
	sum := sha256.Sum256([]byte("hello"))
	m := SnapshotManifest{FormatVersion: 1, Origin: SourceOrigin{Kind: "folder", Location: "private-source"}, Files: []SnapshotFile{{Path: "main.typ", Size: 5, SHA256: hex.EncodeToString(sum[:])}}, Directories: []string{}, Fonts: []SnapshotFont{}}
	id, err := ManifestContentID(m)
	if err != nil {
		t.Fatal(err)
	}
	// M02 hashes this content projection, including its original capitalized keys.
	expected := sha256.Sum256([]byte(`{"FormatVersion":1,"Files":[{"path":"main.typ","size":5,"sha256":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"}],"Directories":[],"Fonts":[]}`))
	if id != hex.EncodeToString(expected[:]) {
		t.Fatalf("M02 identity mismatch: %s", id)
	}
	m.ID = id
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
	m.Origin.Location = "different-source"
	same, err := ManifestContentID(m)
	if err != nil || same != id {
		t.Fatalf("provenance changed identity: %s %v", same, err)
	}
	m.Files[0].Path = "../main.typ"
	if err := ValidateManifest(m); err == nil {
		t.Fatal("accepted escaping path")
	}
}

func TestManifestRejectsUnsortedDuplicateAndWrongIdentity(t *testing.T) {
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cases := []SnapshotManifest{
		{FormatVersion: 1, Files: []SnapshotFile{{Path: "z", SHA256: hash}, {Path: "a", SHA256: hash}}, Directories: []string{}, Fonts: []SnapshotFont{}},
		{FormatVersion: 1, Files: []SnapshotFile{{Path: "a", SHA256: hash}, {Path: "a", SHA256: hash}}, Directories: []string{}, Fonts: []SnapshotFont{}},
		{FormatVersion: 2, Files: []SnapshotFile{}, Directories: []string{}, Fonts: []SnapshotFont{}},
		{FormatVersion: 1, ID: hash, Files: []SnapshotFile{}, Directories: []string{}, Fonts: []SnapshotFont{}},
	}
	for i, m := range cases {
		if err := ValidateManifest(m); err == nil {
			t.Fatalf("accepted invalid manifest %d", i)
		}
	}
}

func TestTransportManifestIdentityDoesNotDependOnNativePathRules(t *testing.T) {
	// Linux captures can contain names that a Windows materializer cannot use.
	m := SnapshotManifest{FormatVersion: 1, Files: []SnapshotFile{{Path: "a:b.typ", Size: 0, SHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}}, Directories: []string{}, Fonts: []SnapshotFont{}}
	id, err := ManifestContentID(m)
	if err != nil {
		t.Fatalf("transport imposed a native OS path restriction: %v", err)
	}
	m.ID = id
	if err := ValidateManifest(m); err != nil {
		t.Fatal(err)
	}
}
