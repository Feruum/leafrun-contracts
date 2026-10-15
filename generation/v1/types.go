// Package generationv1 defines the Generation HTTP and event data contract.
package generationv1

import (
	"encoding/json"
	"time"

	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

type Scope = storagev1.Scope
type CreateJobRequest struct {
	Scope                Scope           `json:"scope"`
	SnapshotID           string          `json:"snapshotId"`
	TargetID             string          `json:"targetId"`
	Entrypoint           string          `json:"entrypoint"`
	Parameters           json.RawMessage `json:"parameters,omitempty"`
	ParametersSchemaPath string          `json:"parametersSchemaPath,omitempty"`
}
type Status string

const (
	Queued     Status = "queued"
	Preparing  Status = "preparing"
	Rendering  Status = "rendering"
	Publishing Status = "publishing"
	Cancelling Status = "cancelling"
	Succeeded  Status = "succeeded"
	Failed     Status = "failed"
	Canceled   Status = "canceled"
	Expired    Status = "expired"
)

func (s Status) Terminal() bool {
	return s == Succeeded || s == Failed || s == Canceled || s == Expired
}

type Diagnostic struct {
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Path     string   `json:"path,omitempty"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Hints    []string `json:"hints,omitempty"`
	Trace    []string `json:"trace,omitempty"`
	Raw      string   `json:"raw,omitempty"`
}
type ArtifactBinding struct {
	ArtifactID       string    `json:"artifactId"`
	SnapshotID       string    `json:"snapshotId"`
	TargetID         string    `json:"targetId"`
	RenderAttemptID  string    `json:"renderAttemptId"`
	InputFingerprint string    `json:"inputFingerprint"`
	SHA256           string    `json:"sha256"`
	SizeBytes        int64     `json:"sizeBytes"`
	ExpiresAt        time.Time `json:"expiresAt"`
}
type Job struct {
	JobID             string           `json:"jobId"`
	Scope             Scope            `json:"scope"`
	Status            Status           `json:"status"`
	Phase             string           `json:"phase"`
	Version           int64            `json:"version"`
	SourceSnapshotID  string           `json:"sourceSnapshotId"`
	SourceContentID   string           `json:"sourceContentId,omitempty"`
	DerivedSnapshotID string           `json:"derivedSnapshotId,omitempty"`
	DerivedContentID  string           `json:"derivedContentId,omitempty"`
	TargetID          string           `json:"targetId"`
	Entrypoint        string           `json:"entrypoint"`
	EngineVersion     string           `json:"engineVersion"`
	InputFingerprint  string           `json:"inputFingerprint"`
	CreatedAt         time.Time        `json:"createdAt"`
	UpdatedAt         time.Time        `json:"updatedAt"`
	Deadline          time.Time        `json:"deadline"`
	TerminalAt        *time.Time       `json:"terminalAt,omitempty"`
	Artifact          *ArtifactBinding `json:"artifact,omitempty"`
	FailureCode       string           `json:"failureCode,omitempty"`
	// Diagnostics and Parameters are omitted unless current source-read is allowed.
	Diagnostics    []Diagnostic    `json:"diagnostics,omitempty"`
	RawDiagnostics string          `json:"rawDiagnostics,omitempty"`
	Parameters     json.RawMessage `json:"parameters,omitempty"`
}
type Admission struct {
	Job                  Job       `json:"job"`
	IdempotencyExpiresAt time.Time `json:"idempotencyExpiresAt"`
}
type FieldError struct {
	File    string `json:"file,omitempty"`
	Pointer string `json:"pointer"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type APIError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// CompletedEvent carries only identifiers/version/state. It intentionally has
// no field for parameter values, raw diagnostics, source files or PDF data.
type CompletedEvent struct {
	JobID      string `json:"jobId"`
	Status     Status `json:"status"`
	ArtifactID string `json:"artifactId,omitempty"`
	SnapshotID string `json:"snapshotId"`
	TargetID   string `json:"targetId"`
}
