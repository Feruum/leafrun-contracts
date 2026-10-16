package ownerproofv1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

func TestE06OwnerProofDisablesAmbientJar(t *testing.T) {
	var cookieSeen, serviceSeen bool
	input := ScopeRequest{Scope: storagev1.Scope{WorkspaceID: "workspace", ProjectID: "project"}, Revision: "revision"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieSeen = r.Header.Get("Cookie") != ""
		serviceSeen = r.Header.Get("Authorization") == "Bearer fixed-service-token-123"
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(RevisionBinding{Scope: input.Scope, Revision: input.Revision, SnapshotID: "snapshot", ContentID: strings.Repeat("a", 64)})
	}))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	target, _ := url.Parse(server.URL)
	jar.SetCookies(target, []*http.Cookie{{Name: "ory_kratos_session", Value: "private-human-session", Path: "/"}})
	client, err := NewClient(server.URL, "fixed-service-token-123", &http.Client{Jar: jar})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.VerifyScope(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if cookieSeen || !serviceSeen {
		t.Fatalf("technical verifier credential boundary violated: ambient_cookie=%v fixed_service=%v", cookieSeen, serviceSeen)
	}
}
