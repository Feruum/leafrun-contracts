package accessv1

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// SessionClient uses explicit user credentials only. It has no fixed service
// credential and shares the internal client's bounded strict response sender.
type SessionClient struct {
	sender          *Client
	cookieName      string
	guestCookieName string
	origin          string
}

// NewSessionClientWithOrigin configures the canonical browser origin explicitly;
// dependency URLs and request Host are never used to infer it.
func NewSessionClientWithOrigin(baseURL, cookieName, origin string, loopbackHTTP bool, transport *http.Client) (*SessionClient, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Host != strings.ToLower(u.Host) || strings.ContainsAny(origin, "\x00\t\r\n ,") || len(origin) > 2048 {
		return nil, ErrInvalid
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || !loopbackHTTP || (u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) {
			return nil, ErrInvalid
		}
	}
	c, err := NewSessionClient(baseURL, cookieName, transport)
	if err != nil {
		return nil, err
	}
	c.origin = origin
	return c, nil
}

func NewSessionClient(baseURL, cookieName string, transport *http.Client) (*SessionClient, error) {
	if cookieName == "" || (&http.Cookie{Name: cookieName, Value: "x"}).Valid() != nil {
		return nil, errors.New("invalid session cookie name")
	}
	sender, err := newSender(baseURL, transport)
	if err != nil {
		return nil, err
	}
	return &SessionClient{sender: sender, cookieName: cookieName}, nil
}
func (c *SessionClient) Session(ctx context.Context, in ResolveRequest) (ResolveResponse, error) {
	if in.Validate() != nil {
		return ResolveResponse{}, ErrUnauthenticated
	}
	if in.SessionCookie != "" {
		for i := 0; i < len(in.SessionCookie); i++ {
			b := in.SessionCookie[i]
			if b < 0x21 || b > 0x7e || strings.ContainsRune("\"\\;,", rune(b)) {
				return ResolveResponse{}, ErrUnauthenticated
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.sender.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.sender.baseURL+"/v1/session", nil)
	if err != nil {
		return ResolveResponse{}, ErrDependency
	}
	if in.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+in.Bearer)
	} else {
		name, e := c.selectedCookieName(in)
		if e != nil {
			return ResolveResponse{}, e
		}
		req.Header.Set("Cookie", name+"="+in.SessionCookie)
	}
	var out ResolveResponse
	if err = c.sender.send(req, &out); err != nil {
		return ResolveResponse{}, err
	}
	if out.Validate(c.sender.now()) != nil {
		return ResolveResponse{}, ErrDependency
	}
	return out, nil
}

// NewSessionClientWithCookieNames maps the closed kind to validated server-agreed cookie names.
func NewSessionClientWithCookieNames(baseURL, human, guest, origin string, loopbackHTTP bool, transport *http.Client) (*SessionClient, error) {
	if human == guest || len(human) > 128 || len(guest) > 128 || guest == "" || (&http.Cookie{Name: guest, Value: "x"}).Valid() != nil {
		return nil, ErrInvalid
	}
	c, err := NewSessionClientWithOrigin(baseURL, human, origin, loopbackHTTP, transport)
	if err != nil {
		return nil, err
	}
	c.guestCookieName = guest
	return c, nil
}
func (c *SessionClient) selectedCookieName(in ResolveRequest) (string, error) {
	if in.SessionCookieKind == CookieKindGuest {
		if c.guestCookieName == "" {
			return "", ErrInvalid
		}
		return c.guestCookieName, nil
	}
	return c.cookieName, nil
}
