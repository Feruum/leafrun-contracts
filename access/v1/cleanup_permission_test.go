package accessv1

import (
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"testing"
)

func TestGenerationCleanupIsClosedServiceOnlyOperation(t *testing.T) {
	r := CheckRequest{Principal: identityv1.Principal{Domain: "service", Subject: "leafrun-generation"}, Scope: PermissionScope{OrganizationID: "org", WorkspaceID: "ws", ProjectID: "project"}, Operation: "generation.cleanup"}
	if r.Validate() != nil {
		t.Fatal("configured service cleanup not admitted")
	}
	if relation, ok := Permit(r.Operation); !ok || relation != "" {
		t.Fatal("cleanup gained human relation")
	}
	for _, domain := range []string{"human", "user", "bot", "guest"} {
		r.Principal.Domain = domain
		if r.Validate() == nil {
			t.Fatal("public actor admitted service cleanup")
		}
	}
	if GrantOperation("generation.cleanup") || BotOperation("generation.cleanup") {
		t.Fatal("public credential can delegate cleanup")
	}
	if _, err := GrantOperations("", []string{"generation.cleanup"}); err == nil {
		t.Fatal("cleanup in grant ceiling")
	}
}
