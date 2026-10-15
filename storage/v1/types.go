// Package storagev1 defines the versioned Storage wire contract, without database models.
package storagev1

import "time"

type Scope struct {
	WorkspaceID string `json:"workspaceId"`
	ProjectID   string `json:"projectId"`
}

type ObjectClass string

const (
	ClassResource ObjectClass = "resource"
	ClassSnapshot ObjectClass = "snapshot"
	ClassResult   ObjectClass = "result"
	ClassRecovery ObjectClass = "recovery"
)

type SourceOrigin struct {
	Kind          string `json:"kind"`
	Location      string `json:"location"`
	SourceRoot    string `json:"sourceRoot"`
	Ref           string `json:"ref"`
	Revision      string `json:"revision"`
	ArchiveSHA256 string `json:"archiveSHA256"`
}
type SnapshotFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}
type SnapshotFont struct {
	Path        string         `json:"path"`
	SourcePath  string         `json:"sourcePath,omitempty"`
	InProject   bool           `json:"inProject"`
	Files       []SnapshotFile `json:"files"`
	Directories []string       `json:"directories"`
}
type SnapshotManifest struct {
	FormatVersion int            `json:"formatVersion"`
	ID            string         `json:"id"`
	Origin        SourceOrigin   `json:"origin"`
	Files         []SnapshotFile `json:"files"`
	Directories   []string       `json:"directories"`
	Fonts         []SnapshotFont `json:"fonts"`
}
type CreateUploadRequest struct {
	Scope          Scope       `json:"scope"`
	Class          ObjectClass `json:"class"`
	SizeBytes      int64       `json:"sizeBytes"`
	SHA256         string      `json:"sha256"`
	IdempotencyKey string      `json:"idempotencyKey"`
}
type Upload struct {
	UploadID    string      `json:"uploadId"`
	Scope       Scope       `json:"scope"`
	Class       ObjectClass `json:"class"`
	State       string      `json:"state"`
	SizeBytes   int64       `json:"sizeBytes"`
	SHA256      string      `json:"sha256"`
	WriterToken string      `json:"writerToken,omitempty"`
	LeaseUntil  time.Time   `json:"leaseUntil"`
	CreatedAt   time.Time   `json:"createdAt"`
	ResourceID  string      `json:"resourceId,omitempty"`
	ArtifactID  string      `json:"artifactId,omitempty"`
}
type CommitResourceRequest struct {
	WriterToken    string `json:"writerToken"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type RenewUploadRequest struct {
	WriterToken string `json:"writerToken"`
}
type Resource struct {
	ResourceID string      `json:"resourceId"`
	Scope      Scope       `json:"scope"`
	Class      ObjectClass `json:"class"`
	SHA256     string      `json:"sha256"`
	SizeBytes  int64       `json:"sizeBytes"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// Tree is "project" or "font"; FontIndex selects an external manifest font for Tree="font".
type ResourceRef struct {
	Tree       string `json:"tree"`
	FontIndex  int    `json:"fontIndex,omitempty"`
	Path       string `json:"path"`
	ResourceID string `json:"resourceId"`
}
type CreateSnapshotRequest struct {
	Scope          Scope            `json:"scope"`
	Manifest       SnapshotManifest `json:"manifest"`
	Resources      []ResourceRef    `json:"resources"`
	IdempotencyKey string           `json:"idempotencyKey"`
}
type Snapshot struct {
	SnapshotID string           `json:"snapshotId"`
	Scope      Scope            `json:"scope"`
	Manifest   SnapshotManifest `json:"manifest"`
	Resources  []ResourceRef    `json:"resources"`
	CreatedAt  time.Time        `json:"createdAt"`
}
type ArtifactMetadata struct {
	Filename         string `json:"filename"`
	MediaType        string `json:"mediaType"`
	TargetID         string `json:"targetId"`
	AttemptID        string `json:"attemptId"`
	Revision         string `json:"revision,omitempty"`
	InputFingerprint string `json:"inputFingerprint,omitempty"`
	DiagnosticsRef   string `json:"diagnosticsRef,omitempty"`
}
type FinalizeArtifactRequest struct {
	WriterToken    string           `json:"writerToken"`
	OriginJobID    string           `json:"originJobId"`
	SnapshotID     string           `json:"snapshotId"`
	Metadata       ArtifactMetadata `json:"metadata"`
	IdempotencyKey string           `json:"idempotencyKey"`
}
type Artifact struct {
	ArtifactID  string           `json:"artifactId"`
	Scope       Scope            `json:"scope"`
	OriginJobID string           `json:"originJobId"`
	SnapshotID  string           `json:"snapshotId"`
	Hash        string           `json:"hash"`
	SizeBytes   int64            `json:"sizeBytes"`
	Metadata    ArtifactMetadata `json:"metadata"`
	CreatedAt   time.Time        `json:"createdAt"`
	ExpiresAt   time.Time        `json:"expiresAt"`
}
type AcquirePinRequest struct {
	Scope           Scope  `json:"scope"`
	ObjectID        string `json:"objectId"`
	HolderID        string `json:"holderId"`
	HolderDomain    string `json:"holderDomain"`
	RetentionPolicy string `json:"retentionPolicy"`
	IdempotencyKey  string `json:"idempotencyKey"`
}
type Pin struct {
	PinID           string    `json:"pinId"`
	Scope           Scope     `json:"scope"`
	ObjectID        string    `json:"objectId"`
	HolderID        string    `json:"holderId"`
	HolderDomain    string    `json:"holderDomain"`
	RetentionPolicy string    `json:"retentionPolicy"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"createdAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

// PinReleaseRequest names the caller's expected reference. A bound release
// checks current permission on this scope before confirming its absence.
type PinReleaseRequest struct {
	Scope           Scope  `json:"scope"`
	ObjectID        string `json:"objectId"`
	HolderID        string `json:"holderId"`
	HolderDomain    string `json:"holderDomain"`
	RetentionPolicy string `json:"retentionPolicy"`
}

type PinReleaseState string

const (
	PinReleaseReleased PinReleaseState = "released"
	PinReleaseAbsent   PinReleaseState = "absent"
)

type PinReleaseReceipt struct {
	PinID   string            `json:"pinId"`
	Binding PinReleaseRequest `json:"binding"`
	State   PinReleaseState   `json:"state"`
}
type Principal struct {
	Subject string `json:"subject"`
	Domain  string `json:"domain"`
}
type AccessCheck struct {
	Principal Principal `json:"principal"`
	Scope     Scope     `json:"scope"`
	Operation string    `json:"operation"`
	ObjectID  string    `json:"objectId,omitempty"`
}
type AccessDecision struct {
	Allowed bool `json:"allowed"`
}
type PublishabilityCheck struct {
	Scope       Scope  `json:"scope"`
	OriginJobID string `json:"originJobId"`
	ArtifactID  string `json:"artifactId"`
}
type PublishabilityDecision struct {
	Publishable bool `json:"publishable"`
}
type PinOwnerCheck struct {
	Scope        Scope  `json:"scope"`
	PinID        string `json:"pinId"`
	ObjectID     string `json:"objectId"`
	HolderID     string `json:"holderId"`
	HolderDomain string `json:"holderDomain"`
}
type OwnerState string

const (
	OwnerReferenced OwnerState = "referenced"
	OwnerAbsent     OwnerState = "absent"
	OwnerUnknown    OwnerState = "unknown"
)

type PinOwnerDecision struct {
	State OwnerState `json:"state"`
}
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type ErrorResponse struct {
	Error APIError `json:"error"`
}
