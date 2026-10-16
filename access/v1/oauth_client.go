package accessv1

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// OAuthBrowserClient constructs fixed HTML/form requests only. It does not send
// requests, retain credentials, parse continuations, or follow redirects. Browser
// callers supply their cookie jar and must enforce no-follow on other transports.
// Existing SessionClient bearer authentication is independent of this surface.
type OAuthBrowserClient struct{ origin string }

func NewOAuthBrowserClient(origin string) (*OAuthBrowserClient, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || len(origin) > 2048 || strings.ContainsAny(origin, "\x00\t\r\n ,#") {
		return nil, ErrInvalid
	}
	return &OAuthBrowserClient{origin: origin}, nil
}

func (c *OAuthBrowserClient) LoginRequest(ctx context.Context, challenge string) (*http.Request, error) {
	return c.challengeRequest(ctx, "login", challenge)
}
func (c *OAuthBrowserClient) ConsentRequest(ctx context.Context, challenge string) (*http.Request, error) {
	return c.challengeRequest(ctx, "consent", challenge)
}
func (c *OAuthBrowserClient) ResumeRequest(ctx context.Context) (*http.Request, error) {
	return c.request(ctx, http.MethodGet, "/oauth/resume", nil)
}
func (c *OAuthBrowserClient) LoginPostRequest(ctx context.Context, form OAuthBrowserForm) (*http.Request, error) {
	return c.postRequest(ctx, "login", form, nil)
}
func (c *OAuthBrowserClient) ConsentPostRequest(ctx context.Context, form OAuthBrowserForm, requested []string) (*http.Request, error) {
	return c.postRequest(ctx, "consent", form, requested)
}

func (c *OAuthBrowserClient) challengeRequest(ctx context.Context, purpose, challenge string) (*http.Request, error) {
	if len(challenge) == 0 || len(challenge) > OAuthChallengeLimit {
		return nil, ErrInvalid
	}
	for _, ch := range challenge {
		if ch < 0x21 || ch > 0x7e || ch == ',' || ch == ';' {
			return nil, ErrInvalid
		}
	}
	return c.request(ctx, http.MethodGet, "/oauth/"+purpose+"?"+url.Values{purpose + "_challenge": {challenge}}.Encode(), nil)
}
func (c *OAuthBrowserClient) postRequest(ctx context.Context, purpose string, form OAuthBrowserForm, requested []string) (*http.Request, error) {
	if form.Validate(purpose, requested) != nil {
		return nil, ErrInvalid
	}
	values := url.Values{"state": {form.State}, "csrf": {form.CSRF}, "action": {form.Action}}
	if form.Action == "accept" {
		for _, scope := range form.Scopes {
			values.Add("scope", scope)
		}
	}
	return c.request(ctx, http.MethodPost, "/oauth/"+purpose, values)
}
func (c *OAuthBrowserClient) request(ctx context.Context, method, path string, values url.Values) (*http.Request, error) {
	var body *strings.Reader
	if method == http.MethodPost {
		body = strings.NewReader(values.Encode())
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.origin+path, body)
	if err != nil {
		return nil, ErrInvalid
	}
	req.Header.Set("Accept", "text/html")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", c.origin)
	}
	return req, nil
}
