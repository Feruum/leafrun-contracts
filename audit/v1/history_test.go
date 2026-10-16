package auditv1

import (
	accessv1 "github.com/leafrun/leafrun-contracts/access/v1"
	"testing"
)

func TestHistoryQueryRequiresCompleteExactScope(t *testing.T) {
	q := HistoryQuery{Scope: accessv1.PermissionScope{OrganizationID: "org", WorkspaceID: "ws", ProjectID: "project"}, Limit: 25}
	if q.Validate() != nil {
		t.Fatal("valid query rejected")
	}
	q.Scope.ProjectID = ""
	if q.Validate() == nil {
		t.Fatal("partial scope accepted")
	}
	q.Scope.ProjectID = "project"
	q.Limit = 101
	if q.Validate() == nil {
		t.Fatal("unbounded limit")
	}
}
