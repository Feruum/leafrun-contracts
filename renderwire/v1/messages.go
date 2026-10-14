package renderwire

import (
	"encoding/json"

	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

// RenderRequest precedes the immutable base bundle. The renderer reads schemas
// only from that base, validates parameters, and then derives input.json.
type RenderRequest struct {
	Scope                storagev1.Scope `json:"scope"`
	JobID                string          `json:"jobId"`
	AttemptID            string          `json:"attemptId"`
	TargetID             string          `json:"targetId"`
	Entrypoint           string          `json:"entrypoint"`
	SourceSnapshotID     string          `json:"sourceSnapshotId"`
	SourceContentID      string          `json:"sourceContentId"`
	InputFingerprint     string          `json:"inputFingerprint"`
	Parameters           json.RawMessage `json:"parameters,omitempty"`
	ParametersSchemaPath string          `json:"parametersSchemaPath,omitempty"`
}

type InputTerminal struct {
	Complete bool `json:"complete"`
}
type PDFDescriptor struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// TerminalEnvelope is always the last output frame. Output order is an optional
// derived bundle, PDFHeader/PDF on success, then Terminal. A compile failure can
// retain the derived bundle for diagnostics; validation failure has only Terminal.
// Its declarations must be checked against the verified staged bytes
// and the persisted attempt before anything is uploaded. Exit 0 is also required.
type TerminalEnvelope struct {
	Complete             bool                      `json:"complete"`
	Success              bool                      `json:"success"`
	Scope                storagev1.Scope           `json:"scope"`
	JobID                string                    `json:"jobId"`
	AttemptID            string                    `json:"attemptId"`
	TargetID             string                    `json:"targetId"`
	InputFingerprint     string                    `json:"inputFingerprint"`
	SourceSnapshotID     string                    `json:"sourceSnapshotId"`
	SourceContentID      string                    `json:"sourceContentId"`
	DerivedContentID     string                    `json:"derivedContentId,omitempty"`
	EngineVersion        string                    `json:"engineVersion"`
	PDF                  *PDFDescriptor            `json:"pdf,omitempty"`
	Phase                string                    `json:"phase"`
	FailureCode          string                    `json:"failureCode,omitempty"`
	Diagnostics          []generationv1.Diagnostic `json:"diagnostics,omitempty"`
	RawDiagnostics       string                    `json:"rawDiagnostics,omitempty"`
	DiagnosticsTruncated bool                      `json:"diagnosticsTruncated,omitempty"`
}
