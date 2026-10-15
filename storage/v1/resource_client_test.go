package storagev1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReadsAuthoritativeResourceMetadata(t *testing.T) {
	want := Resource{ResourceID: "resource-1", Scope: Scope{WorkspaceID: "workspace-1", ProjectID: "project-1"}, Class: ClassResource, SHA256: "0123456789abcdef", SizeBytes: 42}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/v1/resources/resource-1" || r.Header.Get("Authorization") != "Bearer fixed-service-token" {
			t.Errorf("unexpected metadata request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "fixed-service-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.GetResource(context.Background(), want.ResourceID)
	if err != nil || got != want {
		t.Fatalf("metadata: %+v, %v", got, err)
	}
}
