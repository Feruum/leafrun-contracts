package projectsv1

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"

	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
)

const ExecutionVersion = "leafrun-project-execution-v1"

// NormalizeExecution fingerprints the fully frozen owner resolution. This is
// separate from the original public selection and is never a public request.
func NormalizeExecution(raw []byte) (out ResolvedExecution, normalized []byte, fingerprint string, err error) {
	normalized, err = jsonvalue.NormalizeLimit(raw, ResponseLimit)
	if err != nil {
		return
	}
	if err = jsonvalue.Unmarshal(normalized, &out, ResponseLimit); err != nil {
		return
	}
	if !ValidScope(out.Scope) || !ValidIdentifier(out.Actor.Subject) || !ValidIdentifier(out.Actor.Domain) || !validDigest(out.SelectionFingerprint) || !validDigest(out.ContentID) || !validDigest(out.Descriptor.Digest) || !validDigest(out.Descriptor.ExecutionDigest) || !ValidIdentifier(out.SnapshotID) || !ValidIdentifier(out.Revision) || out.Descriptor.Kind != "document" || out.Descriptor.SnapshotContentID != out.ContentID {
		err = errors.New("invalid resolved execution")
		return
	}
	sum := sha256.Sum256(append([]byte(ExecutionVersion+"\n"), normalized...))
	fingerprint = hex.EncodeToString(sum[:])
	return
}
