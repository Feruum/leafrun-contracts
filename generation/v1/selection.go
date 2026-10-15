package generationv1

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
)

const SelectionVersion = "leafrun-render-selection-v1"

// FileReference names an authorized Storage resource. Scope, hashes, sizes and
// generated paths are resolved by the owner and never accepted from callers.
type FileReference struct {
	ResourceID string `json:"resourceId"`
	Filename   string `json:"filename"`
}

// RenderSelectionRequest identifies either an immutable published release or
// an owner-registered working snapshot target. It contains no execution fields.
type RenderSelectionRequest struct {
	Scope       Scope                    `json:"scope"`
	ReleaseID   string                   `json:"releaseId,omitempty"`
	SnapshotID  string                   `json:"snapshotId,omitempty"`
	TargetID    string                   `json:"targetId,omitempty"`
	Parameters  json.RawMessage          `json:"parameters,omitempty"`
	FileRefs    map[string]FileReference `json:"fileRefs,omitempty"`
	Profile     string                   `json:"profile,omitempty"`
	CachePolicy string                   `json:"cachePolicy,omitempty"`
}

// NormalizeSelection binds the original public selection, independently of
// the owner's resolved execution fingerprint. Parameter numbers retain their
// exact lexemes. Cache policy records intent; PDF reuse belongs to E15.
func NormalizeSelection(input []byte) (request RenderSelectionRequest, normalized []byte, fingerprint string, err error) {
	normalized, err = jsonvalue.NormalizeObject(input)
	if err != nil {
		return
	}
	if err = jsonvalue.Unmarshal(normalized, &request, jsonvalue.MaxBytes); err != nil {
		return
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(normalized, &fields); err != nil {
		return
	}
	if !identifier(request.Scope.WorkspaceID) || !identifier(request.Scope.ProjectID) {
		err = errors.New("scope is required")
		return
	}
	_, hasRelease := fields["releaseId"]
	_, hasSnapshot := fields["snapshotId"]
	_, hasTarget := fields["targetId"]
	if hasRelease {
		if !identifier(request.ReleaseID) || hasSnapshot || hasTarget {
			err = errors.New("release selection must contain only releaseId")
			return
		}
	} else if !identifier(request.SnapshotID) || !identifier(request.TargetID) {
		err = errors.New("working selection requires snapshotId and targetId")
		return
	}
	if _, present := fields["profile"]; present && !identifier(request.Profile) {
		err = errors.New("profile must be a bounded identifier")
		return
	}
	if len(request.Parameters) == 0 {
		request.Parameters = json.RawMessage(`{}`)
	}
	request.Parameters, err = jsonvalue.NormalizeObject(request.Parameters)
	if err != nil {
		return
	}
	if request.FileRefs == nil {
		if raw, present := fields["fileRefs"]; present && bytes.Equal(raw, []byte("null")) {
			err = errors.New("fileRefs must be an object")
			return
		}
		request.FileRefs = map[string]FileReference{}
	}
	if len(request.FileRefs) > 64 {
		err = errors.New("fileRefs exceeds 64 references")
		return
	}
	for pointer, ref := range request.FileRefs {
		if !ValidFieldPointer(pointer) || !identifier(ref.ResourceID) || !SafeFilename(ref.Filename) {
			err = errors.New("invalid file reference")
			return
		}
	}
	if request.CachePolicy == "" {
		if _, present := fields["cachePolicy"]; present {
			err = errors.New("unsupported cache policy")
			return
		}
		request.CachePolicy = "use"
	}
	if request.CachePolicy != "use" && request.CachePolicy != "refresh" {
		err = errors.New("unsupported cache policy")
		return
	}
	var encoded []byte
	encoded, err = json.Marshal(request)
	if err != nil {
		return
	}
	normalized, err = jsonvalue.NormalizeObject(encoded)
	if err != nil {
		return
	}
	sum := sha256.Sum256(append([]byte(SelectionVersion+"\n"), normalized...))
	fingerprint = hex.EncodeToString(sum[:])
	return
}

// ValidFieldPointer accepts bounded non-root RFC 6901 pointers, checking every
// escape. Instance/schema resolution determines whether the field exists.
func ValidFieldPointer(value string) bool {
	if len(value) == 0 || len(value) > 4096 || value[0] != '/' || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] == '~' {
			i++
			if i >= len(value) || (value[i] != '0' && value[i] != '1') {
				return false
			}
		}
	}
	return true
}

func SafeFilename(value string) bool {
	if !utf8.ValidString(value) || len(value) == 0 || len(value) > 255 || value == "." || value == ".." || strings.TrimSpace(value) != value || strings.ContainsAny(value, "/\\:") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	extension := path.Ext(value)
	if extension != "" {
		if len(extension) < 2 || len(extension) > 17 {
			return false
		}
		for _, r := range extension[1:] {
			if r < '0' || r > '9' {
				if r < 'a' || r > 'z' {
					if r < 'A' || r > 'Z' {
						return false
					}
				}
			}
		}
	}
	return true
}

// GeneratedInputPath is the versioned deterministic file binding shared with
// Core; only an already checked lowercase SHA-256 and safe filename are used.
func GeneratedInputPath(hash, filename string) string {
	return ".leafrun-input/" + hash + path.Ext(filename)
}
