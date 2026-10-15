package projectsv1

import (
	"context"
	"encoding/json"
	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func selectionFixture(t *testing.T) ResolveRequest {
	t.Helper()
	selection, _, fingerprint, err := generationv1.NormalizeSelection([]byte(`{"scope":{"workspaceId":"w","projectId":"p"},"snapshotId":"s","targetId":"invoice"}`))
	if err != nil {
		t.Fatal(err)
	}
	return ResolveRequest{Actor: Principal{Subject: "alice", Domain: "user"}, Selection: selection, SelectionFingerprint: fingerprint}
}
func resolvedFixture(t *testing.T, request ResolveRequest) ResolvedExecution {
	t.Helper()
	return ResolvedExecution{Actor: request.Actor, SelectionFingerprint: request.SelectionFingerprint, Scope: request.Selection.Scope, Revision: "rev1", SnapshotID: "s", ContentID: strings.Repeat("a", 64), Descriptor: TargetDescriptor{TargetID: "invoice", Kind: "document", Entrypoint: "main.typ", InputSchema: "schema.json", SnapshotContentID: strings.Repeat("a", 64), Digest: strings.Repeat("b", 64), ExecutionDigest: strings.Repeat("c", 64), TypstVersion: "0.15.1", Profiles: map[string]Profile{"production": {}}, DefaultProfile: "production", SelectedProfile: "production"}, Prepared: PreparedInput{JSON: json.RawMessage(`{}`), Files: []PreparedFile{}, Inputs: map[string]string{"leafrun-data": "/input.json"}}, InputResources: []InputResource{}}
}

func TestResolvedInputBindsMergedProfileOptions(t *testing.T) {
	request := selectionFixture(t)
	out := resolvedFixture(t, request)
	out.Descriptor.Profiles["production"] = Profile{Inputs: map[string]string{"watermark": "off"}, PDFStandard: "1.7"}
	out.Descriptor.ResolvedProfile = out.Descriptor.Profiles["production"]
	out.Prepared.Inputs["watermark"] = "off"
	if err := ValidateResolved(request, out); err != nil {
		t.Fatalf("valid frozen profile rejected: %v", err)
	}
	out.Prepared.Inputs["watermark"] = "forged"
	if err := ValidateResolved(request, out); err == nil {
		t.Fatal("changed profile input accepted")
	}
}
func TestResolverClientBindsActorAndOriginalSelection(t *testing.T) {
	request := selectionFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/v1/resolve" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer generation-service" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var got ResolveRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil || got.Actor != request.Actor || got.SelectionFingerprint != request.SelectionFingerprint {
			t.Errorf("wrong resolver binding: %+v %v", got, err)
		}
		_ = json.NewEncoder(w).Encode(resolvedFixture(t, request))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "generation-service", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Resolve(context.Background(), request)
	if err != nil || got.SnapshotID != "s" {
		t.Fatalf("resolve: %+v %v", got, err)
	}
}
func TestResolverClientRejectsUnboundAndUnknownResponses(t *testing.T) {
	request := selectionFixture(t)
	for _, mutate := range []func(map[string]any){
		func(x map[string]any) { x["actor"] = Principal{Subject: "mallory", Domain: "user"} },
		func(x map[string]any) { x["selectionFingerprint"] = strings.Repeat("f", 64) },
		func(x map[string]any) { x["snapshotId"] = "foreign" },
		func(x map[string]any) { x["credentials"] = "secret" },
	} {
		data, _ := json.Marshal(resolvedFixture(t, request))
		var response map[string]any
		_ = json.Unmarshal(data, &response)
		mutate(response)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(response) }))
		client, err := NewClient(server.URL, "generation-service", server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Resolve(context.Background(), request)
		server.Close()
		if err == nil {
			t.Fatal("unbound/unknown response accepted")
		}
	}
}
func TestProjectsClientRefusesRedirects(t *testing.T) {
	calls := 0
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(200) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "private-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Resolve(context.Background(), selectionFixture(t))
	if err == nil || calls != 0 {
		t.Fatalf("redirect followed: calls=%d error=%v", calls, err)
	}
}

func TestProjectsResolverRejectsUnexpectedSuccessStatus(t *testing.T) {
	request := selectionFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		_ = json.NewEncoder(w).Encode(resolvedFixture(t, request))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "generation-service", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Resolve(context.Background(), request); err == nil {
		t.Fatal("pending success accepted as completed owner resolution")
	}
}
