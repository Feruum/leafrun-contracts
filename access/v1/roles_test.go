package accessv1

import "testing"

func TestBindingAndCeilingClosed(t *testing.T) {
	for _, relation := range []string{"reader", "commenter", "editor", "owner", "generate", "download", "source.read", "resource.read"} {
		if (BindingRequest{SubjectKind: "team", SubjectID: "same-team", Relation: relation}).Validate() != nil {
			t.Fatal("valid binding denied")
		}
	}
	for _, relation := range []string{"admin", "members", "parent", "can_generate", "*"} {
		if (BindingRequest{SubjectKind: "human", SubjectID: "actor", Relation: relation}).Validate() == nil {
			t.Fatal("unsafe raw relation")
		}
	}
	scope := PermissionScope{OrganizationID: "org", WorkspaceID: "workspace", ProjectID: "project"}
	ceiling := &CredentialCeiling{Scope: scope, Operations: []string{"generation.create"}}
	if !ceiling.Allows(scope, "generation.create") || ceiling.Allows(scope, "project.read") || ceiling.Allows(PermissionScope{}, "generation.create") {
		t.Fatal("credential ceiling broadened authority")
	}
}
