package generationv1

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type clientRoundTripFunc func(*http.Request) (*http.Response, error)

func (f clientRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGenerationClientSuppliesBoundedDeadline(t *testing.T) {
	transport := &http.Client{Transport: clientRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 30*time.Second {
			t.Error("missing bounded client deadline")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"jobId":"j"}`)), Header: http.Header{}}, nil
	})}
	client, err := NewClient("http://example.test", "token", transport)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetJob(context.Background(), "j"); err != nil {
		t.Fatal(err)
	}
}
func TestGenerationClientRejectsMalformedResponseRecords(t *testing.T) {
	for _, body := range []string{`{"jobId":"j","forged":true}`, `{"jobId":"j","jobId":"other"}`, `{"jobId":"j"} {}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
		client, _ := NewClient(server.URL, "token", server.Client())
		_, err := client.GetJob(context.Background(), "j")
		server.Close()
		if err == nil {
			t.Fatalf("malformed response accepted: %s", body)
		}
	}
}
func TestGenerationClientPreservesFieldFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		_, _ = io.WriteString(w, `{"error":{"code":"parameters_invalid","message":"Parameters are invalid","fields":[{"pointer":"/customer/name","code":"required","message":"Required"}]}}`)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "token", server.Client())
	_, err := client.GetJob(context.Background(), "j")
	var failure *HTTPError
	if !errors.As(err, &failure) || failure.Status != 422 || len(failure.Fields) != 1 || failure.Fields[0].Pointer != "/customer/name" {
		t.Fatalf("field error lost: %+v", failure)
	}
}
