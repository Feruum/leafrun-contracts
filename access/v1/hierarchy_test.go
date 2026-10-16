package accessv1

import (
	"reflect"
	"testing"
	"time"
)

func TestHierarchyPublicContractSurface(t *testing.T) {
	paths := NewOpenAPI()["paths"].(map[string]any)
	for _, path := range []string{"/v1/organizations", "/v1/organizations/{organization}/workspaces", "/v1/workspaces/{workspace}/projects", "/v1/invitations/accept"} {
		if _, ok := paths[path]; !ok {
			t.Errorf("missing documented hierarchy path %s", path)
		}
	}
	for _, name := range []string{"CreateOrganization", "Organizations", "CreateWorkspace", "CreateTeam", "CreateProject", "Projects", "SetOrganizationMembership", "IssueInvitation", "AcceptInvitation"} {
		if _, ok := reflect.TypeFor[*SessionClient]().MethodByName(name); !ok {
			t.Errorf("missing typed client method %s", name)
		}
	}
}
func TestHierarchyRequestValidation(t *testing.T) {
	for _, r := range []ProjectRequest{{Name: "n", Kind: "personal", OwnerID: "human"}, {Name: "n", Kind: "team", TeamID: "team"}} {
		if r.Validate() != nil {
			t.Fatal("valid project")
		}
	}
	for _, r := range []ProjectRequest{{Name: "n", Kind: "personal"}, {Name: "n", Kind: "team", OwnerID: "human", TeamID: "team"}, {Name: "n", Kind: "other", OwnerID: "human"}} {
		if r.Validate() == nil {
			t.Fatal("invalid project accepted")
		}
	}
	for _, r := range []InvitationRequest{{HumanID: "x", Role: "owner", ExpiresAt: time.Now().Add(-time.Second)}, {HumanID: "x", Role: "member", ExpiresAt: time.Now().Add(8 * 24 * time.Hour)}} {
		if r.Validate(time.Now()) == nil {
			t.Fatal("unbounded invitation")
		}
	}
	for _, p := range []PageRequest{{Limit: 0}, {Limit: 101}, {Limit: 1, Cursor: string(make([]byte, 2049))}} {
		if p.Validate() == nil {
			t.Fatal("unbounded page")
		}
	}
}
