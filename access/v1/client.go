package accessv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
)

const ResponseLimit int64 = 64 << 10
const ClientTimeout = 5 * time.Second

var (
	ErrUnauthenticated = errors.New("invalid authentication")
	ErrDenied          = errors.New("access denied")
	ErrDependency      = errors.New("Access dependency uncertainty")
)

// HTTPError deliberately excludes response text, credentials and request URLs.
type HTTPError struct {
	Status int
	Class  error
	cause  error
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("Access request failed (%d): %s", e.Status, e.Class)
}
func (e *HTTPError) Is(target error) bool {
	return errors.Is(e.Class, target) || errors.Is(e.cause, target)
}

type Client struct {
	baseURL, serviceBearer string
	http                   *http.Client
	timeout                time.Duration
	now                    func() time.Time
}

func NewClient(baseURL, serviceBearer string, transport *http.Client) (*Client, error) {
	if !ValidCredential(serviceBearer) {
		return nil, errors.New("invalid internal service credential")
	}
	c, err := newSender(baseURL, transport)
	if err != nil {
		return nil, err
	}
	c.serviceBearer = serviceBearer
	return c, nil
}
func newSender(baseURL string, transport *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(baseURL, "#\r\n") {
		return nil, errors.New("invalid Access API URL")
	}
	if transport == nil {
		transport = http.DefaultClient
	}
	copy := *transport
	copy.Jar = nil // Never add ambient user credentials to explicit transport.
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &copy, timeout: ClientTimeout, now: time.Now}, nil
}
func (c *Client) Resolve(ctx context.Context, in ResolveRequest) (ResolveResponse, error) {
	var out ResolveResponse
	if in.Validate() != nil || in.Bearer == c.serviceBearer || in.SessionCookie == c.serviceBearer {
		return out, ErrUnauthenticated
	}
	if err := c.call(ctx, "/internal/v1/auth/resolve", in, &out); err != nil {
		return ResolveResponse{}, err
	}
	if err := out.Validate(c.now()); err != nil {
		return ResolveResponse{}, &HTTPError{Class: ErrDependency}
	}
	return out, nil
}
func (c *Client) Check(ctx context.Context, in CheckRequest) (bool, error) {
	out, err := c.check(ctx, in, false)
	return out.Allowed, err
}

// CheckScope requires the complete immutable registry hierarchy on an allowed
// check. Callers must retain it with asynchronous operations and recheck that
// exact target; it does not extend the lifetime of an authorization decision.
func (c *Client) CheckScope(ctx context.Context, in CheckRequest) (CheckResponse, error) {
	return c.check(ctx, in, true)
}
func (c *Client) check(ctx context.Context, in CheckRequest, requireScope bool) (CheckResponse, error) {
	if err := in.Validate(); err != nil {
		return CheckResponse{}, err
	}
	// A missing or null boolean is malformed transport, not a valid denial.
	var out struct {
		Allowed        *bool            `json:"allowed"`
		CanonicalScope *PermissionScope `json:"canonicalScope,omitempty"`
	}
	if err := c.call(ctx, "/internal/v1/checks/access", in, &out); err != nil {
		return CheckResponse{}, err
	}
	if out.Allowed == nil {
		return CheckResponse{}, &HTTPError{Class: ErrDependency}
	}
	if out.CanonicalScope != nil {
		s := *out.CanonicalScope
		if !*out.Allowed || s.Validate() != nil || (in.Scope.OrganizationID != "" && s.OrganizationID != in.Scope.OrganizationID) || s.WorkspaceID != in.Scope.WorkspaceID || s.ProjectID != in.Scope.ProjectID {
			return CheckResponse{}, &HTTPError{Class: ErrDependency}
		}
	}
	if requireScope && *out.Allowed && out.CanonicalScope == nil && in.Operation != "health" {
		return CheckResponse{}, &HTTPError{Class: ErrDependency}
	}
	return CheckResponse{Allowed: *out.Allowed, CanonicalScope: out.CanonicalScope}, nil
}
func (c *Client) call(ctx context.Context, path string, in, out any) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	data, err := json.Marshal(in)
	if err != nil {
		return &HTTPError{Class: ErrDependency}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return &HTTPError{Class: ErrDependency}
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceBearer)
	req.Header.Set("Content-Type", "application/json")
	return c.send(req, out)
}
func (c *Client) send(req *http.Request, out any) error {
	return c.sendBounded(req, out, ResponseLimit)
}
func (c *Client) sendBounded(req *http.Request, out any, limit int64) error {
	response, err := c.http.Do(req)
	if err != nil {
		// Preserve only context classification; url.Error and transport messages may
		// contain URLs, caller tokens or upstream response details.
		var cause error
		if errors.Is(err, context.Canceled) {
			cause = context.Canceled
		} else if errors.Is(err, context.DeadlineExceeded) {
			cause = context.DeadlineExceeded
		}
		return &HTTPError{Class: ErrDependency, cause: cause}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		return &HTTPError{Class: ErrDependency}
	}
	if response.StatusCode != http.StatusOK {
		class := ErrDependency
		switch response.StatusCode {
		case http.StatusBadRequest:
			class = ErrInvalid
		case http.StatusConflict:
			class = ErrConflict
		case http.StatusGone:
			class = ErrOperationExpired
		case http.StatusUnauthorized:
			class = ErrUnauthenticated
		case http.StatusForbidden:
			class = ErrDenied
		}
		return &HTTPError{Status: response.StatusCode, Class: class}
	}
	if err := jsonvalue.Unmarshal(body, out, limit); err != nil {
		return &HTTPError{Class: ErrDependency}
	}
	return nil
}
