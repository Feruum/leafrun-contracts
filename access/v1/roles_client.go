package accessv1

import "context"

func (c *SessionClient) Bind(ctx context.Context, credential ResolveRequest, scope PermissionScope, binding BindingRequest, remove bool) (Result, error) {
	if scope.Validate() != nil || binding.Validate() != nil {
		return Result{}, ErrInvalid
	}
	path := "/v1/organizations/" + hierarchySegment(scope.OrganizationID) + "/bindings"
	if scope.WorkspaceID != "" {
		path = "/v1/workspaces/" + hierarchySegment(scope.WorkspaceID)
		if scope.ProjectID != "" {
			path += "/projects/" + hierarchySegment(scope.ProjectID)
		}
		path += "/bindings"
	}
	method := "PUT"
	if remove {
		method = "DELETE"
	}
	var out Result
	e := c.publicCall(ctx, credential, method, path, binding, &out)
	if e != nil {
		return Result{}, e
	}
	if !out.Applied {
		return Result{}, ErrDependency
	}
	return out, nil
}
