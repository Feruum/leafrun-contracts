// Package ownerproofv1 defines narrow, fixed-service legacy-domain evidence.
// Evidence proves domain existence; organization assignment is a separate assertion.
package ownerproofv1

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

const Limit = 4096

var ErrDependency = errors.New("owner evidence unavailable")
var ErrNotFound = errors.New("owner evidence not found")

type ScopeRequest struct {
	Scope    storagev1.Scope `json:"scope"`
	Revision string          `json:"revision"`
}
type SnapshotBinding struct {
	Scope      storagev1.Scope `json:"scope"`
	SnapshotID string          `json:"snapshotId"`
	ContentID  string          `json:"contentId"`
}
type RevisionBinding struct {
	Scope      storagev1.Scope `json:"scope"`
	Revision   string          `json:"revision"`
	SnapshotID string          `json:"snapshotId"`
	ContentID  string          `json:"contentId"`
}
type ImportRequest struct {
	OrganizationID string          `json:"organizationId"`
	Scope          storagev1.Scope `json:"scope"`
	Revision       string          `json:"revision"`
	Kind           string          `json:"kind"`
	OwnerID        string          `json:"ownerId,omitempty"`
	TeamID         string          `json:"teamId,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey"`
}
type ImportResult struct {
	OrganizationID string          `json:"organizationId"`
	Binding        RevisionBinding `json:"binding"`
	Kind           string          `json:"kind"`
	OwnerID        string          `json:"ownerId,omitempty"`
	TeamID         string          `json:"teamId,omitempty"`
}

func Text(s string) bool {
	return len(s) > 0 && len(s) <= 256 && utf8.ValidString(s) && strings.TrimSpace(s) == s && !strings.ContainsAny(s, "\x00\r\n")
}
func Scope(s storagev1.Scope) bool { return Text(s.WorkspaceID) && Text(s.ProjectID) }
func Digest(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == s
}
func (r ScopeRequest) Validate() error {
	if !Scope(r.Scope) || !Text(r.Revision) {
		return ErrNotFound
	}
	return nil
}
func (r SnapshotBinding) Validate() error {
	if !Scope(r.Scope) || !Text(r.SnapshotID) || !Digest(r.ContentID) {
		return ErrNotFound
	}
	return nil
}
func (r RevisionBinding) Validate() error {
	if (ScopeRequest{r.Scope, r.Revision}).Validate() != nil || (SnapshotBinding{r.Scope, r.SnapshotID, r.ContentID}).Validate() != nil {
		return ErrNotFound
	}
	return nil
}
func (r ImportRequest) Validate() error {
	if !identityv1.ValidIdentifier(r.OrganizationID) || (ScopeRequest{r.Scope, r.Revision}).Validate() != nil || !Text(r.IdempotencyKey) {
		return ErrNotFound
	}
	if r.Kind == "personal" && identityv1.ValidIdentifier(r.OwnerID) && r.TeamID == "" {
		return nil
	}
	if r.Kind == "team" && identityv1.ValidIdentifier(r.TeamID) && r.OwnerID == "" {
		return nil
	}
	return ErrNotFound
}

type ScopeVerifier interface {
	VerifyScope(context.Context, ScopeRequest) (RevisionBinding, error)
}
type SnapshotVerifier interface {
	VerifySnapshot(context.Context, SnapshotBinding) (SnapshotBinding, error)
}
type Client struct {
	base, token string
	http        *http.Client
}

func (c *Client) Import(ctx context.Context, in ImportRequest) (ImportResult, error) {
	var out ImportResult
	if in.Validate() != nil {
		return out, ErrNotFound
	}
	if err := c.call(ctx, "/internal/v1/scopes/import", in, &out); err != nil {
		return ImportResult{}, err
	}
	if out.OrganizationID != in.OrganizationID || out.Kind != in.Kind || out.OwnerID != in.OwnerID || out.TeamID != in.TeamID || out.Binding.Validate() != nil || out.Binding.Scope != in.Scope || out.Binding.Revision != in.Revision {
		return ImportResult{}, ErrDependency
	}
	return out, nil
}

func NewClient(base, token string, transport *http.Client) (*Client, error) {
	u, e := url.Parse(base)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(base, "\r\n") || len(token) < 16 || len(token) > 4096 || strings.ContainsAny(token, " \t\r\n\x00") {
		return nil, ErrDependency
	}
	h := http.Client{Timeout: 3 * time.Second}
	if transport != nil {
		h = *transport
		if h.Timeout == 0 || h.Timeout > 3*time.Second {
			h.Timeout = 3 * time.Second
		}
	}
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	h.Jar = nil
	return &Client{strings.TrimRight(base, "/"), token, &h}, nil
}
func (c *Client) call(ctx context.Context, path string, in, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	body, e := json.Marshal(in)
	if e != nil || len(body) > Limit {
		return ErrDependency
	}
	r, e := http.NewRequestWithContext(ctx, "POST", c.base+path, bytes.NewReader(body))
	if e != nil {
		return ErrDependency
	}
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Content-Type", "application/json")
	response, e := c.http.Do(r)
	if e != nil {
		return ErrDependency
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return ErrNotFound
	}
	if response.StatusCode != 200 {
		return ErrDependency
	}
	media, _, e := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if e != nil || len(response.Header.Values("Content-Type")) != 1 || media != "application/json" {
		return ErrDependency
	}
	data, e := io.ReadAll(io.LimitReader(response.Body, Limit+1))
	if e != nil || jsonvalue.Unmarshal(data, out, Limit) != nil {
		return ErrDependency
	}
	return nil
}
func (c *Client) VerifyScope(ctx context.Context, in ScopeRequest) (RevisionBinding, error) {
	var out RevisionBinding
	if in.Validate() != nil {
		return out, ErrNotFound
	}
	if e := c.call(ctx, "/internal/v1/scopes/verify", in, &out); e != nil {
		return RevisionBinding{}, e
	}
	if out.Validate() != nil || out.Scope != in.Scope || out.Revision != in.Revision {
		return RevisionBinding{}, ErrDependency
	}
	return out, nil
}
func (c *Client) VerifySnapshot(ctx context.Context, in SnapshotBinding) (SnapshotBinding, error) {
	var out SnapshotBinding
	if in.Validate() != nil {
		return out, ErrNotFound
	}
	if e := c.call(ctx, "/internal/v1/snapshot-bindings/verify", in, &out); e != nil {
		return SnapshotBinding{}, e
	}
	if out.Validate() != nil || out != in {
		return SnapshotBinding{}, ErrDependency
	}
	return out, nil
}
