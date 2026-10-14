package generationv1

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"

	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
)

// NormalizeRequest rejects structural HTTP errors, preserves exact normalized
// parameter bytes and fingerprints all request fields including project scope.
// Schema validation is deferred and is not performed here.
func NormalizeRequest(input []byte) (request CreateJobRequest, normalized []byte, fingerprint string, err error) {
	normalized, err = jsonvalue.NormalizeObject(input)
	if err != nil {
		return
	}
	err = jsonvalue.Unmarshal(normalized, &request, jsonvalue.MaxBytes)
	if err != nil {
		return
	}
	if !identifier(request.Scope.WorkspaceID) || !identifier(request.Scope.ProjectID) || !identifier(request.SnapshotID) || !identifier(request.TargetID) {
		err = errors.New("scope, snapshotId and targetId are required bounded identifiers")
		return
	}
	if !LogicalFilePath(request.Entrypoint) {
		err = errors.New("entrypoint must be a logical file path")
		return
	}
	hasParameters := len(request.Parameters) > 0
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(normalized, &fields); err != nil {
		return
	}
	_, hasSchema := fields["parametersSchemaPath"]
	if hasParameters != hasSchema {
		err = errors.New("parameters and parametersSchemaPath must be supplied together")
		return
	}
	if hasParameters {
		if !LogicalFilePath(request.ParametersSchemaPath) {
			err = errors.New("parametersSchemaPath must be a logical file path")
			return
		}
		request.Parameters, err = jsonvalue.NormalizeObject(request.Parameters)
		if err != nil {
			return
		}
	}
	// Re-encode known fields so omitted optional fields have one representation.
	// json.RawMessage contains already-normalized bytes; numbers are never decoded
	// into interface{} or converted through float64.
	encoded, e := json.Marshal(request)
	if e != nil {
		err = e
		return
	}
	normalized, err = jsonvalue.NormalizeObject(encoded)
	if err != nil {
		return
	}
	hash := sha256.New()
	hash.Write([]byte(jsonvalue.Version + "\n"))
	hash.Write(normalized)
	fingerprint = hex.EncodeToString(hash.Sum(nil))
	return
}
func identifier(s string) bool {
	return len(s) > 0 && len(s) <= 256 && !strings.ContainsAny(s, "\x00\r\n") && strings.TrimSpace(s) == s
}
func LogicalFilePath(s string) bool {
	return len(s) > 0 && len(s) <= 4096 && s != "." && fs.ValidPath(s) && !strings.ContainsAny(s, "\\\x00")
}
func ValidIdempotencyKey(s string) bool {
	return len(s) > 0 && len(s) <= 200 && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
