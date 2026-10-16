package auditv1

import (
	"context"
	"errors"
	accessv1 "github.com/leafrun/leafrun-contracts/access/v1"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const HistoryResponseLimit = 1 << 20

type Client struct {
	baseURL, humanCookie, guestCookie string
	http                              *http.Client
}

// NewClient carries explicit public credentials only. It has no ambient cookie
// jar, follows no redirects, and never returns upstream diagnostic text.
func NewClient(baseURL, humanCookie, guestCookie string, transport *http.Client) (*Client, error) {
	u, e := url.Parse(baseURL)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(baseURL, "\r\n#") {
		return nil, accessv1.ErrInvalid
	}
	if humanCookie == "" || humanCookie == guestCookie || len(humanCookie) > 128 || len(guestCookie) > 128 || (&http.Cookie{Name: humanCookie, Value: "x"}).Valid() != nil || guestCookie != "" && (&http.Cookie{Name: guestCookie, Value: "x"}).Valid() != nil {
		return nil, accessv1.ErrInvalid
	}
	if transport == nil {
		transport = http.DefaultClient
	}
	copy := *transport
	copy.Jar = nil
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{strings.TrimRight(baseURL, "/"), humanCookie, guestCookie, &copy}, nil
}
func (c *Client) Actions(ctx context.Context, auth accessv1.ResolveRequest, q HistoryQuery) (HistoryPage, error) {
	if q.Validate() != nil {
		return HistoryPage{}, accessv1.ErrInvalid
	}
	if auth.Validate() != nil {
		return HistoryPage{}, accessv1.ErrUnauthenticated
	}
	values := url.Values{"organizationId": {q.Scope.OrganizationID}, "workspaceId": {q.Scope.WorkspaceID}, "projectId": {q.Scope.ProjectID}, "limit": {strconv.Itoa(q.Limit)}}
	for k, v := range map[string]string{"cursor": q.Cursor, "producer": string(q.Producer), "action": string(q.Action), "resourceId": q.ResourceID} {
		if v != "" {
			values.Set(k, v)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/audit/actions?"+values.Encode(), nil)
	if e != nil {
		return HistoryPage{}, accessv1.ErrDependency
	}
	if auth.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Bearer)
	} else {
		name := c.humanCookie
		if auth.SessionCookieKind == accessv1.CookieKindGuest {
			name = c.guestCookie
		}
		cookie := &http.Cookie{Name: name, Value: auth.SessionCookie}
		if name == "" || cookie.Valid() != nil {
			return HistoryPage{}, accessv1.ErrUnauthenticated
		}
		req.Header.Set("Cookie", cookie.String())
	}
	response, e := c.http.Do(req)
	if e != nil {
		if errors.Is(e, context.Canceled) {
			return HistoryPage{}, context.Canceled
		}
		if errors.Is(e, context.DeadlineExceeded) {
			return HistoryPage{}, context.DeadlineExceeded
		}
		return HistoryPage{}, accessv1.ErrDependency
	}
	defer response.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(response.Body, HistoryResponseLimit+1))
	if e != nil || len(raw) > HistoryResponseLimit {
		return HistoryPage{}, accessv1.ErrDependency
	}
	if response.StatusCode != 200 {
		switch response.StatusCode {
		case 400:
			return HistoryPage{}, accessv1.ErrInvalid
		case 401:
			return HistoryPage{}, accessv1.ErrUnauthenticated
		case 403:
			return HistoryPage{}, accessv1.ErrDenied
		}
		return HistoryPage{}, accessv1.ErrDependency
	}
	var page HistoryPage
	if jsonvalue.Unmarshal(raw, &page, HistoryResponseLimit) != nil || page.Items == nil || page.Validate(q) != nil {
		return HistoryPage{}, accessv1.ErrDependency
	}
	return page, nil
}
