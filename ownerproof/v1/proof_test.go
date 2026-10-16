package ownerproofv1

import (
	"context"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnerProofClientRejectsUntrustedSuccess(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"scope":{"workspaceId":"foreign","projectId":"project"},"revision":"r1","snapshotId":"snapshot","contentId":"` + strings.Repeat("a", 64) + `"}`, `{"Scope":{}}`, strings.Repeat("a", 4097)} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(body))
		}))
		client, err := NewClient(server.URL, "dedicated-secret-123456", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.VerifyScope(context.Background(), ScopeRequest{Scope: storagev1.Scope{WorkspaceID: "workspace", ProjectID: "project"}, Revision: "r1"})
		server.Close()
		if err == nil {
			t.Fatalf("untrusted response accepted: %.40s", body)
		}
	}
}
func TestOwnerProofPreservesExactTextAndBinding(t *testing.T) {
	want := RevisionBinding{Scope: storagev1.Scope{WorkspaceID: "old workspace/é", ProjectID: "shared-project"}, Revision: "r1", SnapshotID: "snapshot", ContentID: strings.Repeat("b", 64)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/scopes/verify" || r.Header.Get("Authorization") != "Bearer dedicated-secret-123456" {
			t.Error("wrong fixed protocol")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"scope":{"workspaceId":"old workspace/é","projectId":"shared-project"},"revision":"r1","snapshotId":"snapshot","contentId":"` + want.ContentID + `"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "dedicated-secret-123456", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.VerifyScope(context.Background(), ScopeRequest{Scope: want.Scope, Revision: want.Revision})
	if err != nil || got != want {
		t.Fatalf("binding %+v %v", got, err)
	}
}
