package generationv1

import (
	"bytes"
	"testing"
)

func TestSelectionNormalizesOriginalInputsAndKeepsNumbers(t *testing.T) {
	raw := []byte(`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r1","parameters":{"amount":9007199254740993,"rate":1.00},"fileRefs":{"/logo":{"resourceId":"file1","filename":"logo.svg"}}}`)
	request, normalized, fingerprint, err := NormalizeSelection(raw)
	if err != nil {
		t.Fatal(err)
	}
	if request.ReleaseID != "r1" || request.CachePolicy != "use" || fingerprint == "" || !bytes.Contains(normalized, []byte(`9007199254740993`)) || !bytes.Contains(normalized, []byte(`1.00`)) {
		t.Fatalf("selection changed: %s, %+v", normalized, request)
	}
	_, again, againHash, err := NormalizeSelection(normalized)
	if err != nil || !bytes.Equal(normalized, again) || fingerprint != againHash {
		t.Fatalf("not stable: %s %v", again, err)
	}
	_, _, changedHash, err := NormalizeSelection(bytes.Replace(raw, []byte(`logo.svg`), []byte(`other.svg`), 1))
	if err != nil || changedHash == fingerprint {
		t.Fatalf("file name not bound: %v", err)
	}
}

func TestSelectionRejectsAmbiguityAndServerOwnedInputs(t *testing.T) {
	for _, raw := range []string{
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","snapshotId":"s","targetId":"t"}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"snapshotId":"s"}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","targetId":"t"}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","preparedInput":{}}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","descriptorDigest":"fake"}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","parameters":null}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","fileRefs":null}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","snapshotId":null}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"snapshotId":"s","targetId":"t","releaseId":""}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","profile":null}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","profile":""}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","cachePolicy":""}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","fileRefs":{"/a~2":{"resourceId":"f","filename":"a.svg"}}}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","fileRefs":{"/a":{"resourceId":"f","filename":"../a.svg"}}}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","fileRefs":{"/a":{"resourceId":"f","filename":"a.svg","sha256":"forged"}}}`,
		`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","profile":"x","profile":"y"}`,
	} {
		if _, _, _, err := NormalizeSelection([]byte(raw)); err == nil {
			t.Errorf("accepted invalid selection: %s", raw)
		}
	}
}

func TestSelectionWorkingAndEmptyDefaults(t *testing.T) {
	request, data, _, err := NormalizeSelection([]byte(`{"scope":{"workspaceId":"w","projectId":"p"},"snapshotId":"s","targetId":"a","profile":"draft","cachePolicy":"refresh"}`))
	if err != nil || request.SnapshotID != "s" || string(request.Parameters) != "{}" || len(request.FileRefs) != 0 {
		t.Fatalf("working selection: %s, %v", data, err)
	}
	_, _, _, err = NormalizeSelection([]byte(`{"scope":{"workspaceId":"w","projectId":"p"},"releaseId":"r","cachePolicy":"force"}`))
	if err == nil {
		t.Fatal("unknown cache policy accepted")
	}
}
