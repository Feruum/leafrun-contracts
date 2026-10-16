package accessv1

import (
	"context"
	"errors"
	"fmt"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const validResolveJSON = `{"identity":{"principal":{"subject":"alice","domain":"user"},"authority":{"kind":"kratos_session","id":"session-1","version":1}},"expiresAt":"2099-01-01T00:00:00Z"}`

func checkInput() CheckRequest {
	return CheckRequest{Principal: identityv1.Principal{Subject: "alice", Domain: "user"}, Scope: PermissionScope{WorkspaceID: "w", ProjectID: "p"}, Operation: "resource.read"}
}
func TestClientResolveAndCheck(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer service-secret" || r.Method != "POST" {
			t.Error("wrong internal credential/method")
		}
		switch r.URL.Path {
		case "/internal/v1/auth/resolve":
			fmt.Fprint(w, validResolveJSON)
		case "/internal/v1/checks/access":
			fmt.Fprint(w, `{"allowed":false}`)
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer s.Close()
	c, err := NewClient(s.URL, "service-secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.now = func() time.Time { return time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC) }
	got, err := c.Resolve(context.Background(), ResolveRequest{Bearer: "caller-secret"})
	if err != nil || got.Identity.Principal.Subject != "alice" {
		t.Fatalf("resolve: %#v %v", got, err)
	}
	allowed, err := c.Check(context.Background(), checkInput())
	if err != nil || allowed {
		t.Fatalf("false must be valid: %v %v", allowed, err)
	}
}
func TestClientStrictBoundedResponses(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"allowed":null}`, `{"allowed":true,"allowed":false}`, `{"Allowed":true}`, `{"allowed":true,"secret":"body-secret"}`, `{"allowed":true,"body-secret":1}`, `{"allowed":true} {}`, `{"allowed":true}` + strings.Repeat(" ", int(ResponseLimit)), "{\"allowed\":true,\"x\":\"\xff\"}"} {
		t.Run(fmt.Sprintf("check_%d", len(body)), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer s.Close()
			c, _ := NewClient(s.URL, "service-secret", nil)
			allowed, err := c.Check(context.Background(), checkInput())
			if allowed || !errors.Is(err, ErrDependency) || strings.Contains(err.Error(), "body-secret") {
				t.Fatalf("accepted invalid transport: %v %v", allowed, err)
			}
		})
	}
	for _, body := range []string{`{}`, `null`, strings.Replace(validResolveJSON, `"expiresAt":"2099-01-01T00:00:00Z"`, `"expiresAt":"2000-01-01T00:00:00Z"`, 1), strings.Replace(validResolveJSON, `"version":1`, `"version":9223372036854775808`, 1), strings.Replace(validResolveJSON, `"subject":"alice"`, `"subject":""`, 1), strings.Replace(validResolveJSON, `"identity":`, `"identity":null,"unused":`, 1), strings.Replace(validResolveJSON, `"authority":{`, `"authority":{"token":"body-secret",`, 1)} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		c, _ := NewClient(s.URL, "service-secret", nil)
		out, err := c.Resolve(context.Background(), ResolveRequest{Bearer: "caller-secret"})
		s.Close()
		if !errors.Is(err, ErrDependency) || out.Identity.Principal.Subject != "" {
			t.Fatalf("bad identity accepted: %v %#v", err, out)
		}
	}
}

func TestClientExactWireLimitAndOverflow(t *testing.T) {
	for _, extra := range []int{0, 1} {
		body := `{"allowed":true}` + strings.Repeat(" ", int(ResponseLimit)-len(`{"allowed":true}`)+extra)
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		c, _ := NewClient(s.URL, "service-secret", nil)
		allowed, err := c.Check(context.Background(), checkInput())
		s.Close()
		if extra == 0 && (err != nil || !allowed) {
			t.Fatalf("exact bound rejected: %v", err)
		}
		if extra == 1 && (!errors.Is(err, ErrDependency) || allowed) {
			t.Fatal("one byte over wire bound accepted")
		}
	}
	body := strings.Replace(validResolveJSON, `"version":1`, `"version":18446744073709551616`, 1)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer s.Close()
	c, _ := NewClient(s.URL, "service-secret", nil)
	if _, err := c.Resolve(context.Background(), ResolveRequest{Bearer: "caller-secret"}); !errors.Is(err, ErrDependency) {
		t.Fatal("uint64 overflow accepted")
	}
}
func TestClientClassifiesStatusWithoutSecrets(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   error
	}{{401, ErrUnauthenticated}, {403, ErrDenied}, {503, ErrDependency}, {500, ErrDependency}, {302, ErrDependency}} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			fmt.Fprint(w, "body-secret caller-secret service-secret")
		}))
		c, _ := NewClient(s.URL, "service-secret", nil)
		_, err := c.Resolve(context.Background(), ResolveRequest{Bearer: "caller-secret"})
		s.Close()
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d: %v", tc.status, err)
		}
		for _, secret := range []string{"body-secret", "caller-secret", "service-secret"} {
			if strings.Contains(err.Error(), secret) {
				t.Fatal("secret leaked")
			}
		}
	}
}

func TestClientNonexpiringBotAndMandatorySessionExpiry(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		expiry string
		valid  bool
	}{{"bot_key", "", true}, {"service", "", true}, {"dev", "", true}, {"kratos_session", "", false}, {"guest_session", "", false}, {"oauth_token", "", false}, {"bot_key", `,"expiresAt":"2000-01-01T00:00:00Z"`, false}} {
		body := `{"identity":{"principal":{"subject":"actor","domain":"domain"},"authority":{"kind":"` + tc.kind + `","id":"authority-1","version":1}}` + tc.expiry + `}`
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		c, _ := NewClient(s.URL, "service-secret", nil)
		_, err := c.Resolve(context.Background(), ResolveRequest{Bearer: "caller-secret"})
		s.Close()
		if (err == nil) != tc.valid {
			t.Fatalf("kind %s expiry %s: %v", tc.kind, tc.expiry, err)
		}
	}
}
func TestClientCancellationTimeoutAndRedirect(t *testing.T) {
	var leaks atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaks.Add(1) }))
	defer target.Close()
	release := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/v1/auth/resolve" {
			http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
			return
		}
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer s.Close()
	defer close(release)
	c, _ := NewClient(s.URL, "service-secret", nil)
	if _, err := c.Resolve(context.Background(), ResolveRequest{Bearer: "caller-secret"}); !errors.Is(err, ErrDependency) {
		t.Fatal(err)
	}
	if leaks.Load() != 0 {
		t.Fatal("redirect forwarded credential")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Check(ctx, checkInput()); !errors.Is(err, ErrDependency) || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	c.timeout = 10 * time.Millisecond
	if _, err := c.Check(context.Background(), checkInput()); !errors.Is(err, ErrDependency) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
func TestClientRejectsInvalidConfigurationAndCredentialsBeforeNetwork(t *testing.T) {
	for _, url := range []string{"", "ftp://example.com", "http://user:secret@example.com", "http://example.com?x=secret", "http://example.com#secret"} {
		if _, err := NewClient(url, "service-secret", nil); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal(err)
		}
	}
	if _, err := NewClient("http://example.com", "", nil); err == nil {
		t.Fatal("empty service credential")
	}
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer s.Close()
	c, _ := NewClient(s.URL, "service-secret", nil)
	for _, in := range []ResolveRequest{{Bearer: "service-secret"}, {SessionCookie: "service-secret"}, {Bearer: "a", SessionCookie: "b"}} {
		if _, err := c.Resolve(context.Background(), in); !errors.Is(err, ErrUnauthenticated) {
			t.Fatal(err)
		}
	}
	bad := checkInput()
	bad.Operation = ""
	if _, err := c.Check(context.Background(), bad); err == nil {
		t.Fatal("bad check accepted")
	}
	if calls.Load() != 0 {
		t.Fatal("invalid input sent")
	}
}
