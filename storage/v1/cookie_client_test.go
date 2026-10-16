package storagev1

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExplicitCookieClientGuardsOriginAndInternalRoutes(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Cookie") != "custom_guest=opaque" || r.Header.Get("Origin") != "https://app.example" || r.Header.Get("Authorization") != "" {
			t.Errorf("wrong explicit cookie/origin: %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	}))
	defer s.Close()
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(s.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "human", Value: "ambient"}})
	transport := &http.Client{Jar: jar}
	c, err := NewCookieClient(s.URL, "custom_guest", "opaque", "https://app.example", false, transport)
	if err != nil {
		t.Fatal(err)
	}
	var resp *http.Response
	resp, err = c.request(context.Background(), "POST", "/v1/test", nil, "application/json", "")
	if resp != nil {
		resp.Body.Close()
	}
	if err != nil || calls != 1 {
		t.Fatalf("public cookie failed: %v", err)
	}
	if err := func() error {
		_, err := c.request(context.Background(), "GET", "/internal/v1/resources/x", nil, "", "")
		return err
	}(); err == nil || calls != 1 {
		t.Fatal("internal cookie request escaped")
	}
	if transport.Jar != jar {
		t.Fatal("mutated caller transport")
	}
	for _, origin := range []string{"", "https://app.example/", "http://app.example", "https://app.example?x=1", "https://app.example#x"} {
		if _, err := NewCookieClient(s.URL, "guest", "opaque", origin, false, nil); err == nil {
			t.Fatal("invalid origin accepted")
		}
	}
	if _, err := NewCookieClient(s.URL, "guest", "opaque", "http://127.0.0.1:8080", true, nil); err != nil {
		t.Fatal("explicit loopback denied")
	}
}
