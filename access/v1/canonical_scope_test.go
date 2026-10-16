package accessv1

import (
	"context"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckScopeRequiresExactRegistryHierarchy(t *testing.T) {
	body := `{"allowed":true,"canonicalScope":{"organizationId":"org","workspaceId":"ws","projectId":"project"}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
	defer server.Close()
	client, _ := NewClient(server.URL, "service-secret", nil)
	in := CheckRequest{Principal: identityv1.Principal{Domain: "human", Subject: "actor"}, Scope: PermissionScope{WorkspaceID: "ws", ProjectID: "project"}, Operation: "audit.read"}
	out, err := client.CheckScope(context.Background(), in)
	if err != nil || out.CanonicalScope == nil || out.CanonicalScope.OrganizationID != "org" {
		t.Fatalf("registry scope missing: %v", err)
	}
	for _, bad := range []string{`{"allowed":true}`, `{"allowed":true,"canonicalScope":{"organizationId":"org","workspaceId":"foreign","projectId":"project"}}`, `{"allowed":false,"canonicalScope":{"organizationId":"org"}}`} {
		body = bad
		if _, err := client.CheckScope(context.Background(), in); err == nil {
			t.Fatal("accepted unbound canonical scope")
		}
	}
	body = `{"allowed":true}`
	if allowed, err := client.Check(context.Background(), in); err != nil || !allowed {
		t.Fatal("legacy bool client broken")
	}
}
