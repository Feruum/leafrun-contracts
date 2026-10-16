package accessv1

// Permit is a closed, computed Keto relation. Raw request strings never select
// provider relations. Owner-specific job, publication and TTL gates remain required.
func Permit(operation string) (string, bool) {
	switch operation {
	case "project.read":
		return "isReader", true
	case "comment.create":
		return "isCommenter", true
	case "project.write", "generation.cancel":
		return "isEditor", true
	case "project.publish", "role.manage", "bot.manage", "grant.manage", "audit.read":
		return "isOwner", true
	case "generation.create", "generation.execute", "generation.publish":
		return "can_generate", true
	case "generation.read":
		return "can_status", true
	case "source.read":
		return "can_source_read", true
	case "resource.read":
		return "can_resource_read", true
	case "artifact.read", "download":
		return "can_download", true
	case "organization.read", "workspace.list", "team.read":
		return "can_discover", true
	case "workspace.create", "team.create", "team.member.put", "team.member.delete", "member.put", "member.delete", "invitation.create", "invitation.revoke", "membership.read", "invitation.read", "workspace.role.manage":
		return "can_manage_organization", true
	case "project.create":
		return "can_manage_organization", true
	case "organization.create", "organization.list", "invitation.accept":
		return "", true
	case "health", "upload.create", "upload.get", "snapshot.create", "snapshot.get", "artifact.finalize", "pin.acquire", "pin.get", "pin.release", "generation.cleanup":
		return "", true
	default:
		return "", false
	}
}

// CredentialCeiling is an optional exact operation and hierarchical scope ceiling.
// Nil means unrestricted; a present empty operation set denies everything.
// It can restrict an existing relation but never supplies a relation.
type CredentialCeiling struct {
	Scope      PermissionScope `json:"scope"`
	Operations []string        `json:"operations"`
}

func (c *CredentialCeiling) Allows(scope PermissionScope, operation string) bool {
	if c == nil {
		return true
	}
	if c.Scope.OrganizationID != "" && c.Scope.OrganizationID != scope.OrganizationID {
		return false
	}
	if c.Scope.WorkspaceID != "" && c.Scope.WorkspaceID != scope.WorkspaceID {
		return false
	}
	if c.Scope.ProjectID != "" && c.Scope.ProjectID != scope.ProjectID {
		return false
	}
	for _, op := range c.Operations {
		if op == operation {
			return true
		}
	}
	return false
}
