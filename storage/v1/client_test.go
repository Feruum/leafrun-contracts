package storagev1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientSendsAuthenticatedFinalizeAndReportsConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/internal/v1/uploads/upl_test/finalize" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("incorrect request: %s %s", r.Method, r.URL.Path)
		}
		var request FinalizeArtifactRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.IdempotencyKey != "stable" {
			t.Errorf("body: %+v %v", request, err)
		}
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(ErrorResponse{Error: APIError{Code: "idempotency_conflict", Message: "changed inputs"}})
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.FinalizeArtifact(context.Background(), "upl_test", FinalizeArtifactRequest{IdempotencyKey: "stable"})
	var apiError *HTTPError
	if !errors.As(err, &apiError) || apiError.Status != 409 || apiError.Code != "idempotency_conflict" {
		t.Fatalf("lost API failure: %v", err)
	}
}

func TestClientUploadsAndDownloadsStreams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing principal credential")
		}
		if r.Method == "PUT" {
			bytes, err := io.ReadAll(r.Body)
			if err != nil || string(bytes) != "accepted" || r.Header.Get("X-Writer-Token") != "fence" {
				t.Errorf("upload: %q %v", bytes, err)
			}
			_ = json.NewEncoder(w).Encode(Upload{UploadID: "upl_test", State: "uploaded"})
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = io.WriteString(w, "%PDF-accepted")
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret", server.Client())
	upload, err := client.PutUpload(context.Background(), "upl_test", "fence", strings.NewReader("accepted"))
	if err != nil || upload.State != "uploaded" {
		t.Fatalf("upload response: %+v %v", upload, err)
	}
	response, err := client.ArtifactContent(context.Background(), "art_test")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || string(data) != "%PDF-accepted" {
		t.Fatalf("stream: %s %v", data, err)
	}
}

func TestClientRejectsCredentialsInBaseURL(t *testing.T) {
	if _, err := NewClient("https://user:password@example.com", "", nil); err == nil {
		t.Fatal("accepted URL credentials")
	}
}

func TestClientDoesNotFollowDownloadRedirect(t *testing.T) {
	called := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer server.Close()
	client, _ := NewClient(server.URL, "secret", server.Client())
	_, err := client.ArtifactContent(context.Background(), "art_test")
	if err == nil || called {
		t.Fatalf("followed redirect or hid status: %v %v", called, err)
	}
}
