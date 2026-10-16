package accessv1

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPublicSessionClientExplicitCredentialTransport(t *testing.T) {
	var calls int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/v1/session" {
			t.Fatal("wrong public route")
		}
		if calls == 1 && r.Header.Get("Authorization") != "Bearer user" {
			t.Fatal("missing explicit bearer")
		}
		if calls == 2 && (r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "ory_kratos_session=opaque=+/") {
			t.Fatal("cookie changed or fixed bearer sent")
		}
		fmt.Fprintf(w, `{"identity":{"principal":{"subject":"human-id","domain":"human"},"authority":{"kind":"kratos_session","id":"authority","version":1}},"expiresAt":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	}))
	defer s.Close()
	c, err := NewSessionClient(s.URL, "ory_kratos_session", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range []ResolveRequest{{Bearer: "user"}, {SessionCookie: "opaque=+/"}} {
		if _, err = c.Session(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	for _, in := range []ResolveRequest{{}, {Bearer: "user", SessionCookie: "cookie"}, {SessionCookie: "é"}, {SessionCookie: `a"b`}} {
		if _, err = c.Session(context.Background(), in); err == nil {
			t.Fatal("invalid transport accepted")
		}
	}
	if calls != 2 {
		t.Fatal("invalid input sent")
	}
	if _, err = NewSessionClient(s.URL, "bad name", nil); err == nil {
		t.Fatal("invalid cookie name")
	}
}
func TestPublicSessionClientStrictResponse(t *testing.T) {
	for _, body := range []string{`{"identity":{}}`, `{"Identity":{}}`, `{} {}`, strings.Repeat("x", int(ResponseLimit)+1)} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		c, _ := NewSessionClient(s.URL, "ory_kratos_session", nil)
		if _, err := c.Session(context.Background(), ResolveRequest{Bearer: "user"}); err == nil {
			t.Fatal("invalid response accepted")
		}
		s.Close()
	}
}
