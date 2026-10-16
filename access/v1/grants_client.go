package accessv1

import (
	"context"
)

func (c *SessionClient) CreateGrant(ctx context.Context, auth ResolveRequest, workspace, project string, in CreateGrantRequest) (GrantResult, error) {
	if !validPath(workspace, project) || in.ValidateShape() != nil {
		return GrantResult{}, ErrInvalid
	}
	out, e := c.grantCall(ctx, auth, "POST", "/v1/workspaces/"+hierarchySegment(workspace)+"/projects/"+hierarchySegment(project)+"/grants", in)
	if e == nil && (out.Grant.Scope.WorkspaceID != workspace || out.Grant.Scope.ProjectID != project || out.Grant.Version != 1 || out.Grant.Status != "active") {
		return GrantResult{}, ErrDependency
	}
	return out, e
}
func (c *SessionClient) RotateGrant(ctx context.Context, auth ResolveRequest, grant string, in RotateGrantRequest) (GrantResult, error) {
	if !validPath(grant) || in.ValidateShape() != nil {
		return GrantResult{}, ErrInvalid
	}
	out, e := c.grantCall(ctx, auth, "POST", "/v1/grants/"+hierarchySegment(grant)+"/rotate", in)
	if e == nil && (out.Grant.GrantID != grant || out.Grant.Version != in.ExpectedVersion+1 || out.Grant.Status != "active") {
		return GrantResult{}, ErrDependency
	}
	return out, e
}
func (c *SessionClient) RevokeGrant(ctx context.Context, auth ResolveRequest, grant string, in RevokeGrantRequest) (GrantResult, error) {
	if !validPath(grant) || in.Validate() != nil {
		return GrantResult{}, ErrInvalid
	}
	out, e := c.grantCall(ctx, auth, "DELETE", "/v1/grants/"+hierarchySegment(grant), in)
	if e == nil && (out.Grant.GrantID != grant || out.Grant.Version != in.ExpectedVersion+1 || out.Grant.Status != "revoked" || out.Disclosure != "unavailable") {
		return GrantResult{}, ErrDependency
	}
	return out, e
}
func (c *SessionClient) grantCall(ctx context.Context, auth ResolveRequest, method, path string, in any) (GrantResult, error) {
	var out GrantResult
	if e := c.publicCall(ctx, auth, method, path, in, &out); e != nil {
		return GrantResult{}, e
	}
	if out.Validate() != nil {
		return GrantResult{}, ErrDependency
	}
	return out, nil
}
