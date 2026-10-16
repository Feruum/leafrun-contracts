package auditv1

import (
	"context"
	"errors"
	accessv1 "github.com/leafrun/leafrun-contracts/access/v1"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHistoryClientStrictSafeCredentialTransport(t *testing.T) {
	body := `{"items":[]}`
	status := 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer public-secret" || r.Method != "GET" || r.URL.Query().Get("organizationId") != "org" {
			t.Error("wrong explicit credential/scope transport")
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	defer server.Close()
	c, e := NewClient(server.URL, "human_cookie", "guest_cookie", nil)
	if e != nil {
		t.Fatal(e)
	}
	q := HistoryQuery{Scope: accessv1.PermissionScope{OrganizationID: "org", WorkspaceID: "ws", ProjectID: "project"}, Limit: 25}
	auth := accessv1.ResolveRequest{Bearer: "public-secret"}
	if _, e = c.Actions(context.Background(), auth, q); e != nil {
		t.Fatal(e)
	}
	for _, raw := range []string{`{"items":[],"SECRET_SENTINEL":true}`, `{"items":[],"items":[]}`, `{"items":[]} {}`, `{"items":null}`} {
		body = raw
		if _, e = c.Actions(context.Background(), auth, q); e == nil || strings.Contains(e.Error(), "SENTINEL") {
			t.Fatal("unsafe strict response")
		}
	}
	status = 403
	body = "TOKEN_SENTINEL"
	if _, e = c.Actions(context.Background(), auth, q); !errors.Is(e, accessv1.ErrDenied) || strings.Contains(e.Error(), "SENTINEL") {
		t.Fatal("unsafe denied response")
	}
}
