package projectsv1

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	httpauth "github.com/leafrun/leafrun-contracts/httpauth/v1"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
)

const ResponseLimit int64 = 12 << 20

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
	return fmt.Sprintf("projects HTTP %d (%s): %s", e.Status, e.Code, e.Message)
}

func NewClient(baseURL, token string, transport *http.Client) (*Client, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.ForceQuery || strings.Contains(baseURL, "#") {
		return nil, errors.New("invalid Projects API URL")
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

func (c *Client) RegisterRevision(ctx context.Context, in RegisterRevisionRequest) (Revision, error) {
	var out Revision
	err := c.call(ctx, "POST", "/v1/revisions", in, &out, http.StatusCreated)
	return out, err
}
func (c *Client) GetRevision(ctx context.Context, scope Scope, revision string) (Revision, error) {
	var out Revision
	err := c.call(ctx, "GET", "/v1/revisions/"+url.PathEscape(revision)+scopeQuery(scope), nil, &out)
	return out, err
}
func (c *Client) DeleteRevision(ctx context.Context, revision string, in DeleteRevisionRequest) error {
	return c.call(ctx, "DELETE", "/v1/revisions/"+url.PathEscape(revision), in, nil, http.StatusNoContent)
}
func (c *Client) CreateRelease(ctx context.Context, in CreateReleaseRequest) (Release, error) {
	var out Release
	err := c.call(ctx, "POST", "/v1/releases", in, &out, http.StatusAccepted)
	return out, err
}
func (c *Client) GetRelease(ctx context.Context, id string) (Release, error) {
	var out Release
	err := c.call(ctx, "GET", "/v1/releases/"+url.PathEscape(id), nil, &out)
	return out, err
}
func (c *Client) SetPublication(ctx context.Context, id string, in PublicationRequest) (Release, error) {
	var out Release
	err := c.call(ctx, "POST", "/v1/releases/"+url.PathEscape(id)+"/publication", in, &out)
	return out, err
}
func (c *Client) DeleteRelease(ctx context.Context, id string, in DeleteReleaseRequest) error {
	return c.call(ctx, "DELETE", "/v1/releases/"+url.PathEscape(id), in, nil, http.StatusNoContent)
}
func (c *Client) Catalog(ctx context.Context, scope Scope) (Catalog, error) {
	var out Catalog
	err := c.call(ctx, "GET", "/v1/releases"+scopeQuery(scope), nil, &out)
	return out, err
}
func (c *Client) Form(ctx context.Context, id string) (ReleaseForm, error) {
	var out ReleaseForm
	err := c.call(ctx, "GET", "/v1/releases/"+url.PathEscape(id)+"/form", nil, &out)
	return out, err
}
func (c *Client) CurrentRelease(ctx context.Context, in CurrentReleaseCheck) (CurrentReleaseDecision, error) {
	var out CurrentReleaseDecision
	err := c.call(ctx, "POST", "/internal/v1/checks/release", in, &out)
	return out, err
}
func (c *Client) Resolve(ctx context.Context, in ResolveRequest) (ResolvedExecution, error) {
	var out ResolvedExecution
	data, err := json.Marshal(in.Selection)
	if err != nil {
		return out, err
	}
	selection, _, fingerprint, err := generationv1.NormalizeSelection(data)
	if err != nil || fingerprint != in.SelectionFingerprint || !ValidIdentifier(in.Actor.Subject) || !ValidIdentifier(in.Actor.Domain) {
		return out, errors.New("invalid owner resolution binding")
	}
	in.Selection = selection
	if err = c.call(ctx, "POST", "/internal/v1/resolve", in, &out); err != nil {
		return ResolvedExecution{}, err
	}
	if err = ValidateResolved(in, out); err != nil {
		return ResolvedExecution{}, err
	}
	return out, nil
}

func scopeQuery(scope Scope) string {
	return "?" + url.Values{"workspaceId": {scope.WorkspaceID}, "projectId": {scope.ProjectID}}.Encode()
}

func (c *Client) call(ctx context.Context, method, path string, in, out any, expected ...int) error {
	status := http.StatusOK
	if len(expected) > 0 {
		status = expected[0]
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var data []byte
	if in != nil {
		var err error
		data, err = json.Marshal(in)
		if err != nil {
			return err
		}
		if int64(len(data)) > ResponseLimit {
			return errors.New("Projects request exceeds limit")
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if c.cookie != nil {
		if err := c.cookie.Apply(req); err != nil {
			return err
		}
	} else {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, ResponseLimit+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > ResponseLimit {
		return errors.New("Projects response exceeds limit")
	}
	if response.StatusCode != status {
		var envelope ErrorResponse
		_ = jsonvalue.Unmarshal(body, &envelope, 64<<10)
		if envelope.Error.Code == "" {
			envelope.Error.Code = "http_error"
			envelope.Error.Message = http.StatusText(response.StatusCode)
		}
		return &HTTPError{Status: response.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message, Fields: envelope.Error.Fields}
	}
	if out == nil {
		if response.StatusCode != 204 || len(body) != 0 {
			return errors.New("invalid empty Projects response")
		}
		return nil
	}
	if response.StatusCode == 204 {
		return errors.New("Projects response omitted receipt")
	}
	return jsonvalue.Unmarshal(body, out, ResponseLimit)
}

func ValidIdentifier(value string) bool {
	return utf8.ValidString(value) && len(value) > 0 && len(value) <= 256 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\x00\r\n")
}
func ValidScope(scope Scope) bool {
	return ValidIdentifier(scope.WorkspaceID) && ValidIdentifier(scope.ProjectID)
}
func validDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// ValidateResolved rejects a response for another actor/selection or a changed
// resource allowlist. Core separately verifies descriptors/schema and bytes.
func ValidateResolved(in ResolveRequest, out ResolvedExecution) error {
	if out.Actor != in.Actor || out.SelectionFingerprint != in.SelectionFingerprint || out.Scope != in.Selection.Scope || !ValidIdentifier(out.Revision) || !ValidIdentifier(out.SnapshotID) || !validDigest(out.ContentID) || out.ReleaseID != in.Selection.ReleaseID {
		return errors.New("Projects resolution identity mismatch")
	}
	if in.Selection.ReleaseID == "" && (out.SnapshotID != in.Selection.SnapshotID || out.Descriptor.TargetID != in.Selection.TargetID) {
		return errors.New("Projects working selection mismatch")
	}
	if out.Descriptor.Kind != "document" || out.Descriptor.SnapshotContentID != out.ContentID || out.Descriptor.TypstVersion != "0.15.1" || !validDigest(out.Descriptor.Digest) || !validDigest(out.Descriptor.ExecutionDigest) || !generationv1.LogicalFilePath(out.Descriptor.Entrypoint) || !generationv1.LogicalFilePath(out.Descriptor.InputSchema) {
		return errors.New("Projects execution descriptor is invalid")
	}
	profileName := in.Selection.Profile
	if profileName == "" {
		profileName = out.Descriptor.DefaultProfile
	}
	profile, exists := out.Descriptor.Profiles[profileName]
	if !ValidIdentifier(profileName) || !exists || out.Descriptor.SelectedProfile != profileName || profile.PDFStandard != out.Descriptor.ResolvedProfile.PDFStandard || !maps.Equal(profile.Inputs, out.Descriptor.ResolvedProfile.Inputs) {
		return errors.New("Projects resolved profile binding mismatch")
	}
	if _, reserved := profile.Inputs["leafrun-data"]; reserved {
		return errors.New("Projects profile overrides reserved input")
	}
	canonical, err := jsonvalue.NormalizeObject(out.Prepared.JSON)
	if err != nil || !bytes.Equal(canonical, out.Prepared.JSON) || out.Prepared.Inputs["leafrun-data"] != "/input.json" || len(out.Prepared.Inputs) != len(profile.Inputs)+1 || len(out.Prepared.Files) > 64 || len(out.InputResources) != len(out.Prepared.Files) {
		return errors.New("Projects prepared input is invalid")
	}
	for key, value := range profile.Inputs {
		if got, present := out.Prepared.Inputs[key]; !present || got != value {
			return errors.New("Projects prepared profile inputs mismatch")
		}
	}
	resources := map[string]InputResource{}
	for _, resource := range out.InputResources {
		if resource.Scope != out.Scope || resource.Class != storagev1.ClassResource || resources[resource.File.Pointer].File.Pointer != "" {
			return errors.New("Projects input resource binding mismatch")
		}
		resources[resource.File.Pointer] = resource
	}
	unique := map[string]PreparedFile{}
	paths := map[string]PreparedFile{}
	var size int64
	for _, file := range out.Prepared.Files {
		ref, exists := in.Selection.FileRefs[file.Pointer]
		if !exists || ref.ResourceID != file.ResourceID || ref.Filename != file.Filename || !generationv1.SafeFilename(file.Filename) || !validDigest(file.SHA256) || file.SizeBytes < 0 || file.SizeBytes > 25<<20 || file.Path != generationv1.GeneratedInputPath(file.SHA256, file.Filename) || resources[file.Pointer].File != file {
			return errors.New("Projects input file binding mismatch")
		}
		if prev, ok := unique[file.ResourceID]; ok {
			if prev.SHA256 != file.SHA256 || prev.SizeBytes != file.SizeBytes {
				return errors.New("inconsistent resource identity")
			}
		} else {
			unique[file.ResourceID] = file
		}
		if previous, ok := paths[file.Path]; ok {
			if previous.SHA256 != file.SHA256 || previous.SizeBytes != file.SizeBytes {
				return errors.New("inconsistent generated input path")
			}
		} else {
			paths[file.Path] = file
			size += file.SizeBytes
		}
		delete(resources, file.Pointer)
	}
	if len(resources) != 0 || len(out.Prepared.Files) != len(in.Selection.FileRefs) || size > 25<<20 {
		return errors.New("Projects input file limits exceeded")
	}
	return nil
}
