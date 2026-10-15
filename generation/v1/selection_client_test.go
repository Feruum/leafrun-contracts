package generationv1

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCreatesRenderSelectionWithDefaultInputs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/renders" || r.Method != "POST" || r.Header.Get("Idempotency-Key") != "same-selection" || r.Header.Get("Authorization") != "Bearer actor-token" {
			t.Errorf("unexpected selection request %s %s", r.Method, r.URL.Path)
		}
		var in RenderSelectionRequest
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ReleaseID != "release1" || string(in.Parameters) != "{}" || in.CachePolicy != "use" {
			t.Errorf("invalid selection body: %+v %v", in, err)
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(Admission{Job: Job{JobID: "job1", Scope: Scope{WorkspaceID: "w", ProjectID: "p"}}})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "actor-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.CreateRender(context.Background(), "same-selection", RenderSelectionRequest{Scope: Scope{WorkspaceID: "w", ProjectID: "p"}, ReleaseID: "release1"})
	if err != nil || got.Job.JobID != "job1" {
		t.Fatalf("create selection: %+v %v", got, err)
	}
}
