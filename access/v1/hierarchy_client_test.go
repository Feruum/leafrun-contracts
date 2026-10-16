package accessv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHierarchyClientConflictClass(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(409); w.Write([]byte("SECRET")) }))
	defer s.Close()
	c, _ := NewSessionClient(s.URL, "session", nil)
	_, e := c.RemoveOrganizationMembership(context.Background(), ResolveRequest{Bearer: "user"}, "org", "human")
	if !errors.Is(e, ErrConflict) || strings.Contains(e.Error(), "SECRET") {
		t.Fatal("conflict not safely classified")
	}
}
func TestHierarchyClientStrictScopeAndResponse(t *testing.T) {
	for _, body := range []string{`{"organizationId":"foreign","workspaceId":"w","name":"ok"}`, `{"organizationId":"org","workspaceId":"w","name":"ok","actor":"injected"}`, `{"organizationId":"org","workspaceId":"w","name":"ok","name":"duplicate"}`, `{"organizationId":"org","name":"missing id"}`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/v1/organizations/org/workspaces" || r.Header.Get("Authorization") != "Bearer user" || r.Header.Get("Cookie") != "" || r.Header.Get("Content-Type") != "application/json" {
				t.Error("wrong explicit public request")
			}
			w.Write([]byte(body))
		}))
		c, _ := NewSessionClient(s.URL, "session", nil)
		_, e := c.CreateWorkspace(context.Background(), ResolveRequest{Bearer: "user"}, "org", NameRequest{Name: "ok"})
		s.Close()
		if !errors.Is(e, ErrDependency) {
			t.Fatal("malformed/foreign response accepted")
		}
	}
}
func TestHierarchyClientTypedCookieAndPage(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "session=user" || r.Header.Get("Authorization") != "" || r.URL.Query().Get("limit") != "1" {
			t.Error("wrong page request")
		}
		w.Write([]byte(`{"items":[{"organizationId":"org","workspaceId":"w","projectId":"p","name":"n","kind":"personal","ownerId":"human"}],"nextCursor":"opaque"}`))
	}))
	defer s.Close()
	c, _ := NewSessionClient(s.URL, "session", nil)
	page, e := c.Projects(context.Background(), ResolveRequest{SessionCookie: "user"}, "w", PageRequest{Limit: 1})
	if e != nil || len(page.Items) != 1 || page.NextCursor != "opaque" {
		t.Fatal("typed cookie page failed")
	}
}
func TestHierarchyClientMaximumEscapedLegacyPage(t *testing.T) {
	id := strings.Repeat("<", 256)
	items := make([]Project, 100)
	for i := range items {
		items[i] = Project{OrganizationID: id, WorkspaceID: id, ProjectID: strings.Repeat("<", 253) + fmt.Sprintf("%03d", i), Name: strings.Repeat("<", 128), Kind: "personal", OwnerID: id}
	}
	var encoded bytes.Buffer
	if json.NewEncoder(&encoded).Encode(Page[Project]{Items: items, NextCursor: strings.Repeat("a", 2048)}) != nil {
		t.Fatal("page encoding failed")
	}
	if encoded.Len() <= int(ResponseLimit) {
		t.Fatal("fixture did not exceed tight auth limit")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(encoded.Bytes()) }))
	defer server.Close()
	c, _ := NewSessionClient(server.URL, "session", nil)
	page, e := c.Projects(context.Background(), ResolveRequest{Bearer: "user"}, id, PageRequest{Limit: 100})
	if e != nil || len(page.Items) != 100 {
		t.Fatalf("valid escaped maximum-width hierarchy page rejected: bytes=%d", encoded.Len())
	}
	if _, e = c.Session(context.Background(), ResolveRequest{Bearer: "user"}); !errors.Is(e, ErrDependency) {
		t.Fatal("authentication response cap was widened")
	}
}
func TestHierarchyClientResponseStillBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat(" ", (1<<20)+1))) }))
	defer server.Close()
	c, _ := NewSessionClient(server.URL, "session", nil)
	if _, e := c.Organizations(context.Background(), ResolveRequest{Bearer: "user"}, PageRequest{Limit: 100}); !errors.Is(e, ErrDependency) {
		t.Fatal("oversized hierarchy body accepted")
	}
}
func TestHierarchyClientLiteralDotSegmentsThroughMux(t *testing.T) {
	for _, id := range []string{".", "..", "a/b", "a?b", "a#b"} {
		t.Run(id, func(t *testing.T) {
			mux := http.NewServeMux()
			reads, deletes := 0, 0
			mux.HandleFunc("GET /v1/workspaces/{workspace}/projects/{project}", func(w http.ResponseWriter, r *http.Request) {
				if r.PathValue("workspace") != id || r.PathValue("project") != id {
					t.Error("read hierarchy ID was normalized")
				}
				reads++
				json.NewEncoder(w).Encode(Project{OrganizationID: "org", WorkspaceID: id, ProjectID: id, Name: "n", Kind: "personal", OwnerID: "human"})
			})
			mux.HandleFunc("DELETE /v1/organizations/{organization}/memberships/{human}", func(w http.ResponseWriter, r *http.Request) {
				if r.PathValue("organization") != id || r.PathValue("human") != id {
					t.Error("destructive hierarchy ID was normalized")
				}
				deletes++
				json.NewEncoder(w).Encode(Result{Applied: true})
			})
			server := httptest.NewServer(mux)
			defer server.Close()
			c, _ := NewSessionClient(server.URL, "session", nil)
			auth := ResolveRequest{Bearer: "user"}
			if _, e := c.Project(context.Background(), auth, id, id); e != nil {
				t.Fatal("literal workspace/project segment was redirected or denied")
			}
			if _, e := c.RemoveOrganizationMembership(context.Background(), auth, id, id); e != nil {
				t.Fatal("literal destructive segment was redirected or denied")
			}
			if reads != 1 || deletes != 1 {
				t.Fatal("exact route not reached once")
			}
		})
	}
}
