package accessv1

import (
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"testing"
)

func TestUnknownPermissionOperationIsClosed(t *testing.T) {
	r := CheckRequest{Principal: identityv1.Principal{Domain: "human", Subject: "actor"}, Scope: PermissionScope{OrganizationID: "org"}, Operation: "future.superuser"}
	if r.Validate() == nil {
		t.Fatal("unknown operation accepted")
	}
}
