package storagev1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) CreateUpload(ctx context.Context, input CreateUploadRequest) (Upload, error) {
	return callJSON[Upload](c, ctx, "POST", "/internal/v1/uploads", input)
}
func (c *Client) GetUpload(ctx context.Context, id string) (Upload, error) {
	return callJSON[Upload](c, ctx, "GET", objectPath("uploads", id), nil)
}
func (c *Client) RenewUpload(ctx context.Context, id string, input RenewUploadRequest) (Upload, error) {
	return callJSON[Upload](c, ctx, "POST", objectPath("uploads", id)+"/renew", input)
}
func (c *Client) PutUpload(ctx context.Context, id, writerToken string, body io.Reader) (Upload, error) {
	var result Upload
	response, err := c.request(ctx, "PUT", objectPath("uploads", id)+"/content", body, "application/octet-stream", writerToken)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result)
	return result, err
}
func (c *Client) CommitResource(ctx context.Context, id string, input CommitResourceRequest) (Resource, error) {
	return callJSON[Resource](c, ctx, "POST", objectPath("uploads", id)+"/commit", input)
}
func (c *Client) CreateSnapshot(ctx context.Context, input CreateSnapshotRequest) (Snapshot, error) {
	return callJSON[Snapshot](c, ctx, "POST", "/internal/v1/snapshots", input)
}
func (c *Client) GetSnapshot(ctx context.Context, id string) (Snapshot, error) {
	return callJSON[Snapshot](c, ctx, "GET", objectPath("snapshots", id), nil)
}
func (c *Client) ResourceContent(ctx context.Context, id string) (*http.Response, error) {
	return c.request(ctx, "GET", objectPath("resources", id)+"/content", nil, "", "")
}
func (c *Client) FinalizeArtifact(ctx context.Context, id string, input FinalizeArtifactRequest) (Artifact, error) {
	return callJSON[Artifact](c, ctx, "POST", objectPath("uploads", id)+"/finalize", input)
}
func (c *Client) GetArtifact(ctx context.Context, id string) (Artifact, error) {
	return callJSON[Artifact](c, ctx, "GET", "/v1/artifacts/"+url.PathEscape(id), nil)
}
func (c *Client) ArtifactContent(ctx context.Context, id string) (*http.Response, error) {
	return c.request(ctx, "GET", "/v1/artifacts/"+url.PathEscape(id)+"/content", nil, "", "")
}
func (c *Client) AcquirePin(ctx context.Context, input AcquirePinRequest) (Pin, error) {
	return callJSON[Pin](c, ctx, "POST", "/internal/v1/pins", input)
}
func (c *Client) GetPin(ctx context.Context, id string) (Pin, error) {
	return callJSON[Pin](c, ctx, "GET", objectPath("pins", id), nil)
}
func (c *Client) ActivatePin(ctx context.Context, id string) (Pin, error) {
	return callJSON[Pin](c, ctx, "POST", objectPath("pins", id)+"/activate", nil)
}
func (c *Client) ReleasePin(ctx context.Context, id string) error {
	response, err := c.request(ctx, "DELETE", objectPath("pins", id), nil, "", "")
	if err != nil {
		return err
	}
	return response.Body.Close()
}
