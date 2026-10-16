package storagev1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	httpauth "github.com/leafrun/leafrun-contracts/httpauth/v1"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	cookie  *httpauth.Cookie
	baseURL string
	token   string
	http    *http.Client
}
type HTTPError struct {
	Status  int
	Code    string
	Message string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("storage HTTP %d (%s): %s", e.Status, e.Code, e.Message)
}

// NewClient disables redirects: requests and credentials remain at the configured Storage API.
func NewClient(baseURL, token string, transport *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid Storage API URL")
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

func (c *Client) request(ctx context.Context, method, path string, body io.Reader, contentType, writerToken string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.cookie != nil {
		if err := c.cookie.Apply(request); err != nil {
			return nil, err
		}
	} else if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if writerToken != "" {
		request.Header.Set("X-Writer-Token", writerToken)
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		var envelope ErrorResponse
		_ = json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&envelope)
		if envelope.Error.Code == "" {
			envelope.Error.Code = "http_error"
			envelope.Error.Message = http.StatusText(response.StatusCode)
		}
		return nil, &HTTPError{Status: response.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message}
	}
	return response, nil
}
func callJSON[T any](c *Client, ctx context.Context, method, path string, input any) (T, error) {
	var result T
	var body io.Reader
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return result, err
		}
		body = bytes.NewReader(data)
	}
	response, err := c.request(ctx, method, path, body, "application/json", "")
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	err = json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&result)
	return result, err
}
func objectPath(kind, id string) string { return "/internal/v1/" + kind + "/" + url.PathEscape(id) }
