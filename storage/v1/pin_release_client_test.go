package storagev1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientReleasesExactPinBinding(t *testing.T) {
	binding := PinReleaseRequest{Scope: Scope{WorkspaceID: "workspace", ProjectID: "project"}, ObjectID: "snapshot", HolderID: "generation", HolderDomain: "service", RetentionPolicy: "ordinary"}
	want := PinReleaseReceipt{PinID: "pin-1", Binding: binding, State: PinReleaseAbsent}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v1/pins/pin-1/release" || r.Header.Get("Authorization") != "Bearer fixed-token" {
			t.Errorf("unexpected bound release: %s %s", r.Method, r.URL.Path)
		}
		var got PinReleaseRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil || got != binding {
			t.Errorf("binding changed: %+v, %v", got, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "fixed-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.ReleasePinBound(context.Background(), want.PinID, binding)
	if err != nil || got != want {
		t.Fatalf("bound release receipt: %+v, %v", got, err)
	}
}

func TestClientRejectsUnconfirmedPinReleaseReceipt(t *testing.T) {
	binding := PinReleaseRequest{Scope: Scope{WorkspaceID: "workspace", ProjectID: "project"}, ObjectID: "snapshot", HolderID: "generation", HolderDomain: "service", RetentionPolicy: "ordinary"}
	for _, mutate := range []func(*PinReleaseReceipt){
		func(r *PinReleaseReceipt) { r.PinID = "different-pin" },
		func(r *PinReleaseReceipt) { r.Binding.ObjectID = "different-object" },
		func(r *PinReleaseReceipt) { r.Binding.Scope.ProjectID = "different-project" },
		func(r *PinReleaseReceipt) { r.State = "unknown" },
	} {
		response := PinReleaseReceipt{PinID: "pin-1", Binding: binding, State: PinReleaseAbsent}
		mutate(&response)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(response) }))
		client, err := NewClient(server.URL, "fixed-token", server.Client())
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.ReleasePinBound(context.Background(), "pin-1", binding)
		server.Close()
		if err == nil {
			t.Fatalf("unconfirmed pin release receipt was accepted: %+v", response)
		}
	}
}
