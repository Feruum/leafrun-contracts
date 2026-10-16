package accessv1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieMutationRequiresExplicitClientOrigin(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Origin") != "https://app.example" {
			t.Error("explicit origin missing")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"organizationId":"organization","name":"Acme"}`))
	}))
	defer server.Close()
	c, _ := NewSessionClient(server.URL, "session", nil)
	if _, err := c.CreateOrganization(context.Background(), ResolveRequest{SessionCookie: "user"}, NameRequest{Name: "Acme"}); err == nil || calls != 0 {
		t.Fatal("unsafe cookie mutation without configured origin reached network")
	}
	c, err := NewSessionClientWithOrigin(server.URL, "session", "https://app.example", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.CreateOrganization(context.Background(), ResolveRequest{SessionCookie: "user"}, NameRequest{Name: "Acme"}); err != nil || calls != 1 {
		t.Fatalf("configured cookie mutation %v calls=%d", err, calls)
	}
	for _, origin := range []string{"*", "null", "https://app.example/path", "https://app.example,https://other.example", "http://app.example"} {
		if _, err = NewSessionClientWithOrigin(server.URL, "session", origin, false, nil); err == nil {
			t.Fatalf("bad origin %s", origin)
		}
	}
}
