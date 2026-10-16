package generationv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	httpauth "github.com/leafrun/leafrun-contracts/httpauth/v1"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	cookie         *httpauth.Cookie
	baseURL, token string
	http           *http.Client
}
type HTTPError struct {
	Status        int
	Code, Message string
	Fields        []FieldError
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("generation HTTP %d (%s): %s", e.Status, e.Code, e.Message)
}
func NewClient(baseURL, token string, transport *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || strings.Contains(baseURL, "#") {
		return nil, errors.New("invalid Generation API URL")
	}
	if transport == nil {
		transport = http.DefaultClient
	}
	copy := *transport
	copy.Jar = nil
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, http: &copy}, nil
}

// NewCookieClient deliberately sends one configured public cookie and the explicit unsafe-request Origin.
// Internal service operations are unavailable through this transport.
func NewCookieClient(baseURL, cookieName, cookieValue, origin string, loopbackHTTP bool, transport *http.Client) (*Client, error) {
	cookie, err := httpauth.NewCookie(cookieName, cookieValue, origin, loopbackHTTP)
	if err != nil {
		return nil, err
	}
	c, err := NewClient(baseURL, "", transport)
	if err != nil {
		return nil, err
	}
	c.cookie = cookie
	return c, nil
}

func (c *Client) CreateJob(ctx context.Context, key string, request CreateJobRequest) (Admission, error) {
	var result Admission
	if !ValidIdempotencyKey(key) {
		return result, errors.New("invalid Idempotency-Key")
	}
	data, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	_, data, _, err = NormalizeRequest(data)
	if err != nil {
		return result, err
	}
	err = c.call(ctx, http.MethodPost, "/v1/jobs", key, data, 202, &result)
	return result, err
}

func (c *Client) CreateRender(ctx context.Context, key string, request RenderSelectionRequest) (Admission, error) {
	var result Admission
	if !ValidIdempotencyKey(key) {
		return result, errors.New("invalid Idempotency-Key")
	}
	data, err := json.Marshal(request)
	if err != nil {
		return result, err
	}
	_, data, _, err = NormalizeSelection(data)
	if err != nil {
		return result, err
	}
	err = c.call(ctx, http.MethodPost, "/v1/renders", key, data, 202, &result)
	return result, err
}
func (c *Client) GetJob(ctx context.Context, id string) (Job, error) {
	var result Job
	if !identifier(id) {
		return result, errors.New("invalid job ID")
	}
	err := c.call(ctx, http.MethodGet, "/v1/jobs/"+url.PathEscape(id), "", nil, 200, &result)
	return result, err
}
func (c *Client) CancelJob(ctx context.Context, id string) (Job, error) {
	var result Job
	if !identifier(id) {
		return result, errors.New("invalid job ID")
	}
	err := c.call(ctx, http.MethodPost, "/v1/jobs/"+url.PathEscape(id)+"/cancel", "", nil, 200, &result)
	return result, err
}
func (c *Client) call(ctx context.Context, method, path, key string, data []byte, status int, result any) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if c.cookie != nil {
		if err := c.cookie.Apply(req); err != nil {
			return err
		}
	} else if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		var envelope ErrorResponse
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
		if readErr != nil || len(body) > 64<<10 || jsonvalue.Unmarshal(body, &envelope, 64<<10) != nil {
			envelope = ErrorResponse{}
		}
		if envelope.Error.Code == "" {
			envelope.Error.Code = "http_error"
			envelope.Error.Message = http.StatusText(resp.StatusCode)
		}
		return &HTTPError{Status: resp.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message, Fields: envelope.Error.Fields}
	}
	const responseLimit = 4 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if err != nil {
		return err
	}
	if len(body) > responseLimit {
		return errors.New("Generation response exceeds limit")
	}
	return jsonvalue.Unmarshal(body, result, responseLimit)
}
