// Package projectsv1 defines immutable project revisions, releases and owner
// resolution. Database records and Storage credentials never cross this API.
package projectsv1

import (
	"encoding/json"
	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
	"time"
)

type Scope = storagev1.Scope
type Principal = storagev1.Principal

type RegisterRevisionRequest struct {
	Scope          Scope  `json:"scope"`
	Revision       string `json:"revision"`
	SnapshotID     string `json:"snapshotId"`
	IdempotencyKey string `json:"idempotencyKey"`
}
type Revision struct {
	Scope      Scope              `json:"scope"`
	Revision   string             `json:"revision"`
	SnapshotID string             `json:"snapshotId"`
	ContentID  string             `json:"contentId"`
	Targets    []TargetDescriptor `json:"targets"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type Profile struct {
	Inputs      map[string]string `json:"inputs,omitempty"`
	PDFStandard string            `json:"pdfStandard,omitempty"`
}
type Output struct {
	Format string `json:"format"`
	Path   string `json:"path"`
}
type Step struct {
	ID           string   `json:"id"`
	Template     string   `json:"template"`
	InputPointer string   `json:"inputPointer"`
	DependsOn    []string `json:"dependsOn,omitempty"`
}
type Target struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Template    string   `json:"template,omitempty"`
	InputSchema string   `json:"inputSchema,omitempty"`
	Steps       []Step   `json:"steps,omitempty"`
	Order       []string `json:"order,omitempty"`
	Output      Output   `json:"output"`
}
type StepDescriptor struct {
	ID           string   `json:"id"`
	TemplateID   string   `json:"templateId"`
	Entrypoint   string   `json:"entrypoint"`
	InputPointer string   `json:"inputPointer"`
	DependsOn    []string `json:"dependsOn"`
}
type ResourceDigest struct {
	Kind      string `json:"kind"`
	FontIndex int    `json:"fontIndex,omitempty"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

// TargetDescriptor mirrors Core's versioned value; Services convert through
// strict JSON so Core and wire representations cannot silently diverge.
type TargetDescriptor struct {
	TargetID          string             `json:"targetId"`
	Kind              string             `json:"kind"`
	TemplateID        string             `json:"templateId,omitempty"`
	Entrypoint        string             `json:"entrypoint,omitempty"`
	InputSchema       string             `json:"inputSchema"`
	Output            Output             `json:"output"`
	Steps             []StepDescriptor   `json:"steps"`
	Profiles          map[string]Profile `json:"profiles"`
	DefaultProfile    string             `json:"defaultProfile"`
	SelectedProfile   string             `json:"selectedProfile"`
	ResolvedProfile   Profile            `json:"resolvedProfile"`
	TypstVersion      string             `json:"typstVersion"`
	SnapshotContentID string             `json:"snapshotContentId"`
	Resources         []ResourceDigest   `json:"resources"`
	Digest            string             `json:"digest"`
	ExecutionDigest   string             `json:"executionDigest"`
	Target            Target             `json:"target"`
}
type CreateReleaseRequest struct {
	Scope          Scope  `json:"scope"`
	Revision       string `json:"revision"`
	TargetID       string `json:"targetId"`
	Version        string `json:"version"`
	IdempotencyKey string `json:"idempotencyKey"`
}

// Release exposes immutable identity and mutable publication separately.
// Source paths/resources remain in protected revision/owner endpoints.
type Release struct {
	ReleaseID          string    `json:"releaseId"`
	Scope              Scope     `json:"scope"`
	Revision           string    `json:"revision"`
	TargetID           string    `json:"targetId"`
	Version            string    `json:"version"`
	SnapshotID         string    `json:"snapshotId"`
	ContentID          string    `json:"contentId"`
	DescriptorDigest   string    `json:"descriptorDigest"`
	State              string    `json:"state"`
	Published          bool      `json:"published"`
	PublicationVersion int64     `json:"publicationVersion"`
	CreatedAt          time.Time `json:"createdAt"`
}
type PublicationRequest struct {
	Scope           Scope `json:"scope"`
	ExpectedVersion int64 `json:"expectedVersion"`
	Published       bool  `json:"published"`
}
type DeleteReleaseRequest struct {
	Scope           Scope `json:"scope"`
	ExpectedVersion int64 `json:"expectedVersion"`
}
type DeleteRevisionRequest struct {
	Scope Scope `json:"scope"`
}
type Catalog struct {
	Releases []Release `json:"releases"`
}
type ReleaseForm struct {
	ReleaseID        string                              `json:"releaseId"`
	DescriptorDigest string                              `json:"descriptorDigest"`
	Schema           json.RawMessage                     `json:"schema"`
	SchemaResources  map[string]json.RawMessage          `json:"schemaResources"`
	DefaultsPolicy   string                              `json:"defaultsPolicy"`
	Profiles         map[string]Profile                  `json:"profiles"`
	DefaultProfile   string                              `json:"defaultProfile"`
	APIExample       generationv1.RenderSelectionRequest `json:"apiExample"`
}
type PreparedFile struct {
	Pointer    string `json:"pointer"`
	ResourceID string `json:"resourceId"`
	Filename   string `json:"filename"`
	SHA256     string `json:"sha256"`
	SizeBytes  int64  `json:"sizeBytes"`
	Path       string `json:"path"`
}
type PreparedInput struct {
	JSON   json.RawMessage   `json:"json"`
	Files  []PreparedFile    `json:"files"`
	Inputs map[string]string `json:"inputs"`
}
type InputResource struct {
	File  PreparedFile          `json:"file"`
	Scope Scope                 `json:"scope"`
	Class storagev1.ObjectClass `json:"class"`
}

// ResolveRequest is accepted only from the fixed Generation service identity.
// Actor must be constructed from server-side authentication, never public JSON.
type ResolveRequest struct {
	Authority            *identityv1.AuthorityRef            `json:"authority,omitempty"`
	Actor                Principal                           `json:"actor"`
	Selection            generationv1.RenderSelectionRequest `json:"selection"`
	SelectionFingerprint string                              `json:"selectionFingerprint"`
}
type ResolvedExecution struct {
	Actor                Principal        `json:"actor"`
	SelectionFingerprint string           `json:"selectionFingerprint"`
	Scope                Scope            `json:"scope"`
	ReleaseID            string           `json:"releaseId,omitempty"`
	Revision             string           `json:"revision"`
	SnapshotID           string           `json:"snapshotId"`
	ContentID            string           `json:"contentId"`
	Descriptor           TargetDescriptor `json:"descriptor"`
	Prepared             PreparedInput    `json:"prepared"`
	InputResources       []InputResource  `json:"inputResources"`
}
type CurrentReleaseCheck struct {
	Authority        *identityv1.AuthorityRef `json:"authority,omitempty"`
	Actor            Principal                `json:"actor"`
	Scope            Scope                    `json:"scope"`
	ReleaseID        string                   `json:"releaseId"`
	DescriptorDigest string                   `json:"descriptorDigest"`
}
type CurrentReleaseDecision struct {
	Allowed            bool  `json:"allowed"`
	PublicationVersion int64 `json:"publicationVersion"`
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
