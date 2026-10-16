package accessv1

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func oauthTestForm() url.Values {
	return url.Values{"state": {"12345678-1234-4234-8234-123456789abc"}, "csrf": {base64.RawURLEncoding.EncodeToString(make([]byte, 32))}, "action": {"accept"}, "scope": {"leafrun.generate"}}
}

func TestOAuthBrowserFormValidatesExactPurposeAndRequestedSubset(t *testing.T) {
	requested := []string{"leafrun.generate", "offline_access"}
	for _, purpose := range []string{"login", "consent"} {
		values := oauthTestForm()
		if purpose == "login" {
			values.Del("scope")
		}
		form, err := ParseOAuthBrowserForm(values, purpose, requested)
		if err != nil || form.Validate(purpose, requested) != nil {
			t.Fatalf("valid %s rejected", purpose)
		}
	}
	values := oauthTestForm()
	values.Set("action", "deny")
	form, err := ParseOAuthBrowserForm(values, "consent", requested)
	if err != nil || form.Scopes != nil {
		t.Fatal("consent deny must discard validated selection")
	}
	values.Del("scope")
	if _, err := ParseOAuthBrowserForm(values, "consent", requested); err != nil {
		t.Fatal("empty deny rejected")
	}
	for _, tc := range []struct {
		name, purpose string
		change        func(url.Values)
	}{
		{"actor override", "consent", func(v url.Values) { v.Set("subject", "other") }},
		{"authority override", "consent", func(v url.Values) { v.Set("authority", "other") }},
		{"client override", "consent", func(v url.Values) { v.Set("client", "other") }},
		{"redirect override", "consent", func(v url.Values) { v.Set("return_to", "https://evil.invalid") }},
		{"duplicate state", "consent", func(v url.Values) { v.Add("state", v.Get("state")) }},
		{"missing state", "consent", func(v url.Values) { v.Del("state") }},
		{"noncanonical UUID", "consent", func(v url.Values) { v.Set("state", strings.ToUpper(v.Get("state"))) }},
		{"invalid UUID", "consent", func(v url.Values) { v.Set("state", "123") }},
		{"duplicate csrf", "consent", func(v url.Values) { v.Add("csrf", v.Get("csrf")) }},
		{"noncanonical csrf", "consent", func(v url.Values) { v.Set("csrf", strings.Repeat("A", 42)+"B") }},
		{"padded csrf", "consent", func(v url.Values) { v.Set("csrf", v.Get("csrf")+"=") }},
		{"duplicate action", "consent", func(v url.Values) { v.Add("action", "deny") }},
		{"unknown action", "consent", func(v url.Values) { v.Set("action", "remember") }},
		{"empty accept", "consent", func(v url.Values) { v.Del("scope") }},
		{"unrequested known scope", "consent", func(v url.Values) { v.Set("scope", "leafrun.download") }},
		{"unknown scope", "consent", func(v url.Values) { v.Set("scope", "admin") }},
		{"repeated scope", "consent", func(v url.Values) { v.Add("scope", "leafrun.generate") }},
		{"too many scopes", "consent", func(v url.Values) { v["scope"] = make([]string, 7) }},
		{"login scope", "login", func(v url.Values) {}},
		{"login deny", "login", func(v url.Values) { v.Del("scope"); v.Set("action", "deny") }},
		{"wrong purpose", "resume", func(v url.Values) {}},
		{"deny unrequested scope", "consent", func(v url.Values) { v.Set("action", "deny"); v.Set("scope", "leafrun.download") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := oauthTestForm()
			tc.change(values)
			if _, err := ParseOAuthBrowserForm(values, tc.purpose, requested); !errors.Is(err, ErrInvalid) {
				t.Fatal("invalid form accepted")
			}
		})
	}
	for _, requested := range [][]string{nil, {"unknown"}, {"leafrun.generate", "leafrun.generate"}} {
		if _, err := ParseOAuthBrowserForm(oauthTestForm(), "consent", requested); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid requested scopes accepted")
		}
	}
}

func TestOAuthBrowserClientConstructsOnlyFixedHTMLFormRequests(t *testing.T) {
	c, err := NewOAuthBrowserClient("https://access.example")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, tc := range []struct{ purpose, path, key string }{{"login", "/oauth/login", "login_challenge"}, {"consent", "/oauth/consent", "consent_challenge"}} {
		var req *http.Request
		if tc.purpose == "login" {
			req, err = c.LoginRequest(ctx, "challenge_123")
		} else {
			req, err = c.ConsentRequest(ctx, "challenge_123")
		}
		if err != nil || req.Method != "GET" || req.URL.Path != tc.path || !reflect.DeepEqual(req.URL.Query(), url.Values{tc.key: {"challenge_123"}}) || req.Header.Get("Accept") != "text/html" {
			t.Fatal("incorrect browser GET")
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" {
			t.Fatal("ambient credentials attached")
		}
	}
	req, err := c.ResumeRequest(ctx)
	if err != nil || req.URL.Path != "/oauth/resume" || req.URL.RawQuery != "" {
		t.Fatal("resume accepts query state")
	}
	form, _ := ParseOAuthBrowserForm(oauthTestForm(), "consent", []string{"leafrun.generate"})
	req, err = c.ConsentPostRequest(ctx, form, []string{"leafrun.generate"})
	if err != nil || req.Method != "POST" || req.URL.RawQuery != "" || req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || req.Header.Get("Origin") != "https://access.example" {
		t.Fatal("incorrect browser POST")
	}
	body, _ := io.ReadAll(req.Body)
	values, _ := url.ParseQuery(string(body))
	if !reflect.DeepEqual(values, oauthTestForm()) {
		t.Fatal("browser form lost exact fields")
	}
	if _, err = c.LoginPostRequest(ctx, form); !errors.Is(err, ErrInvalid) {
		t.Fatal("login accepted scopes")
	}
	form.Scopes = nil
	if _, err = c.LoginPostRequest(ctx, form); err != nil {
		t.Fatal("valid login rejected")
	}
	for _, challenge := range []string{"", "bad challenge", strings.Repeat("a", 8193), "bad\nchallenge"} {
		if _, err = c.LoginRequest(ctx, challenge); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid challenge accepted")
		}
	}
	for _, base := range []string{"https://user:pass@access.example", "https://access.example/path", "https://access.example?return_to=x", "https://access.example/#fragment", "ftp://access.example"} {
		if _, err := NewOAuthBrowserClient(base); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid browser origin accepted")
		}
	}
}
