package accessv1

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestExplicitCredentialsExcludeAmbientJar(t *testing.T) {
	var expected string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != expected {
			t.Errorf("explicit credential gained ambient cookie: %q", got)
		}
		fmt.Fprintf(w, `{"identity":{"principal":{"subject":"human","domain":"human"},"authority":{"kind":"kratos_session","id":"authority","version":1}},"expiresAt":%q}`, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	}))
	defer s.Close()
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(s.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "ory_kratos_session", Value: "ambient"}})
	transport := &http.Client{Jar: jar}
	c, _ := NewSessionClient(s.URL, "ory_kratos_session", transport)
	if _, err := c.Session(context.Background(), ResolveRequest{Bearer: "explicit"}); err != nil {
		t.Fatal(err)
	}
	expected = "ory_kratos_session=selected"
	if _, err := c.Session(context.Background(), ResolveRequest{SessionCookie: "selected"}); err != nil {
		t.Fatal(err)
	}
	expected = ""
	internal, _ := NewClient(s.URL, "service", transport)
	if _, err := internal.Resolve(context.Background(), ResolveRequest{Bearer: "explicit"}); err != nil {
		t.Fatal(err)
	}
	if transport.Jar != jar || len(jar.Cookies(u)) != 1 {
		t.Fatal("caller jar modified")
	}
}
