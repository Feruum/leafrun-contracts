package generationv1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientAdmissionAndJobOperations(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing credentials")
		}
		switch r.URL.Path {
		case "/v1/jobs":
			if r.Method != "POST" || r.Header.Get("Idempotency-Key") != "same" {
				t.Error("missing idempotency")
			}
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), `"parameters":{"n":1.0}`) {
				t.Errorf("changed parameter lexeme: %s", body)
			}
			w.WriteHeader(202)
			fmt.Fprint(w, `{"job":{"jobId":"j","status":"queued"},"idempotencyExpiresAt":"2026-10-21T00:00:00Z"}`)
		case "/v1/jobs/j", "/v1/jobs/j/cancel":
			if r.URL.Path == "/v1/jobs/j/cancel" {
				body, _ := io.ReadAll(r.Body)
				if len(body) != 0 {
					t.Error("bodyless cancellation client sent a body")
				}
			}
			fmt.Fprint(w, `{"jobId":"j","status":"queued"}`)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	request := CreateJobRequest{Scope: Scope{WorkspaceID: "w", ProjectID: "p"}, SnapshotID: "s", TargetID: "t", Entrypoint: "main.typ", Parameters: json.RawMessage(`{"n":1.0}`), ParametersSchemaPath: "schema.json"}
	if _, err = client.CreateJob(context.Background(), "", request); err == nil || calls != 0 {
		t.Fatal("missing key sent")
	}
	admission, err := client.CreateJob(context.Background(), "same", request)
	if err != nil || admission.Job.JobID != "j" {
		t.Fatal(admission, err)
	}
	if _, err = client.GetJob(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	if _, err = client.CancelJob(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatal(calls)
	}
}
func TestClientDoesNotRedirectCredentials(t *testing.T) {
	var leaked bool
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret", server.Client())
	if _, err := client.GetJob(context.Background(), "j"); err == nil || leaked {
		t.Fatal("followed redirect")
	}
}

func TestClientRejectsQueryAndFragmentMarkers(t *testing.T) {
	for _, base := range []string{"http://example.test?", "http://example.test#", "http://example.test/path?", "http://example.test/path#"} {
		if _, err := NewClient(base, "secret", nil); err == nil {
			t.Errorf("accepted %s", base)
		}
	}
}
