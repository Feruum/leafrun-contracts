package accessv1

import (
	"errors"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

// PermissionScope is distinct from the immutable content scope. DTO validation
// checks shape only; Access must compare the supplied hierarchy to its registry.
type PermissionScope struct {
	OrganizationID string `json:"organizationId,omitempty"`
	WorkspaceID    string `json:"workspaceId,omitempty"`
	ProjectID      string `json:"projectId,omitempty"`
}

func (s PermissionScope) Validate() error { return s.validate(false) }

// ValidateLegacy additionally accepts only the exact workspace/project pair with
// omitted organization. Access derives its organization from a unique registry pair.
func (s PermissionScope) ValidateLegacy() error { return s.validate(true) }
func (s PermissionScope) validate(legacy bool) error {
	for _, id := range []string{s.OrganizationID, s.WorkspaceID, s.ProjectID} {
		if id != "" && !identityv1.ValidIdentifier(id) {
			return errors.New("invalid permission scope ID")
		}
	}
	if s.OrganizationID == "" {
		if legacy && s.WorkspaceID != "" && s.ProjectID != "" {
			return nil
		}
		return errors.New("organization is required")
	}
	if s.ProjectID != "" && s.WorkspaceID == "" {
		return errors.New("project requires workspace")
	}
	return nil
}
func FromStorageAccessCheck(in storagev1.AccessCheck) CheckRequest {
	return CheckRequest{Principal: in.Principal, Authority: in.Authority, Scope: PermissionScope{WorkspaceID: in.Scope.WorkspaceID, ProjectID: in.Scope.ProjectID}, Operation: in.Operation, ObjectID: in.ObjectID}
}
