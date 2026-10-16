package accessv1

import (
	"reflect"
	"strings"
)

func hierarchyOpenAPI(b *schemaBuilder, paths map[string]any) {
	add := func(method, path, id string, in, out reflect.Type, list bool) {
		responses := map[string]any{"200": map[string]any{"description": "Success", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(out)}}}}
		for _, code := range []string{"400", "401", "403", "409", "503"} {
			responses[code] = map[string]any{"description": "Bounded invalid request, authentication, denial, invariant conflict or dependency uncertainty; never echoes request/provider data."}
		}
		op := map[string]any{"operationId": id, "security": []any{map[string]any{"userBearer": []string{}}, map[string]any{"sessionCookie": []string{}}}, "responses": responses}
		params := []any{}
		for _, part := range strings.Split(path, "/") {
			if strings.HasPrefix(part, "{") {
				params = append(params, map[string]any{"name": strings.Trim(part, "{}"), "in": "path", "required": true, "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "x-max-utf8-bytes": 256}})
			}
		}
		if list {
			params = append(params, map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 50}}, map[string]any{"name": "cursor", "in": "query", "schema": map[string]any{"type": "string", "maxLength": 2048}, "description": "Opaque keyset continuation bound to this route scope and current human. The SQL scope predicate is independent of its contents."})
		}
		if len(params) > 0 {
			op["parameters"] = params
		}
		if in != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(in)}}}
			op["description"] = "Requires JSON (UTF-8), strict bounded fields and a current human session. Browser Origin, when present, must equal the configured canonical Access origin; cross-site fetches are denied. Initiating actor and IDs are server-derived. Management requires the current organization owner under its row lock."
		}
		item, ok := paths[path].(map[string]any)
		if !ok {
			item = map[string]any{}
			paths[path] = item
		}
		item[method] = op
	}
	for i, path := range []string{"/v1/organizations/{organization}/bindings", "/v1/workspaces/{workspace}/bindings", "/v1/workspaces/{workspace}/projects/{project}/bindings"} {
		for _, method := range []string{"put", "delete"} {
			add(method, path, method+[]string{"OrganizationBinding", "WorkspaceBinding", "ProjectBinding"}[i], reflect.TypeFor[BindingRequest](), reflect.TypeFor[Result](), false)
		}
	}
	binding := b.schemas["BindingRequest"].(map[string]any)["properties"].(map[string]any)
	binding["subjectKind"].(map[string]any)["enum"] = []string{"human", "team", "bot"}
	binding["relation"].(map[string]any)["enum"] = []string{"reader", "commenter", "editor", "owner", "generate", "download", "source.read", "resource.read"}
	for _, r := range []struct {
		path, id string
		typ      reflect.Type
	}{
		{"/v1/organizations", "Organizations", reflect.TypeFor[Page[Organization]]()},
		{"/v1/organizations/{organization}/workspaces", "Workspaces", reflect.TypeFor[Page[Workspace]]()},
		{"/v1/organizations/{organization}/teams", "Teams", reflect.TypeFor[Page[Team]]()},
		{"/v1/organizations/{organization}/memberships", "OrganizationMemberships", reflect.TypeFor[Page[Membership]]()},
		{"/v1/organizations/{organization}/invitations", "Invitations", reflect.TypeFor[Page[Invitation]]()},
		{"/v1/workspaces/{workspace}/projects", "Projects", reflect.TypeFor[Page[Project]]()},
		{"/v1/teams/{team}/memberships", "TeamMemberships", reflect.TypeFor[Page[Membership]]()},
	} {
		add("get", r.path, "list"+r.id, nil, r.typ, true)
	}
	for _, r := range []struct {
		method, path, id string
		in, out          reflect.Type
	}{
		{"post", "/v1/organizations", "createOrganization", reflect.TypeFor[NameRequest](), reflect.TypeFor[Organization]()},
		{"post", "/v1/organizations/{organization}/workspaces", "createWorkspace", reflect.TypeFor[NameRequest](), reflect.TypeFor[Workspace]()},
		{"post", "/v1/organizations/{organization}/teams", "createTeam", reflect.TypeFor[NameRequest](), reflect.TypeFor[Team]()},
		{"post", "/v1/workspaces/{workspace}/projects", "createProject", reflect.TypeFor[ProjectRequest](), reflect.TypeFor[Project]()},
		{"post", "/v1/organizations/{organization}/memberships", "addOrganizationMembership", reflect.TypeFor[MembershipRequest](), reflect.TypeFor[Membership]()},
		{"put", "/v1/organizations/{organization}/memberships/{human}", "setOrganizationMembership", reflect.TypeFor[MembershipRequest](), reflect.TypeFor[Membership]()},
		{"delete", "/v1/organizations/{organization}/memberships/{human}", "removeOrganizationMembership", reflect.TypeFor[struct{}](), reflect.TypeFor[Result]()},
		{"post", "/v1/teams/{team}/memberships", "addTeamMembership", reflect.TypeFor[MembershipRequest](), reflect.TypeFor[Membership]()},
		{"delete", "/v1/teams/{team}/memberships/{human}", "removeTeamMembership", reflect.TypeFor[struct{}](), reflect.TypeFor[Result]()},
		{"post", "/v1/organizations/{organization}/invitations", "issueInvitation", reflect.TypeFor[InvitationRequest](), reflect.TypeFor[IssuedInvitation]()},
		{"delete", "/v1/organizations/{organization}/invitations/{id}", "revokeInvitation", reflect.TypeFor[struct{}](), reflect.TypeFor[Result]()},
		{"post", "/v1/invitations/accept", "acceptInvitation", reflect.TypeFor[AcceptInvitationRequest](), reflect.TypeFor[Result]()},
		{"get", "/v1/organizations/{id}", "getOrganization", nil, reflect.TypeFor[Organization]()},
		{"get", "/v1/organizations/{organization}/workspaces/{id}", "getWorkspace", nil, reflect.TypeFor[Workspace]()},
		{"get", "/v1/organizations/{organization}/teams/{id}", "getTeam", nil, reflect.TypeFor[Team]()},
		{"get", "/v1/workspaces/{workspace}/projects/{id}", "getProject", nil, reflect.TypeFor[Project]()},
		{"get", "/v1/organizations/{organization}/invitations/{id}", "getInvitation", nil, reflect.TypeFor[Invitation]()},
	} {
		add(r.method, r.path, r.id, r.in, r.out, false)
	}
	// An anonymous empty request must have a stable nonempty schema name.
	if empty, ok := b.schemas[""]; ok {
		b.schemas["EmptyRequest"] = empty
		delete(b.schemas, "")
		for _, item := range paths {
			for _, raw := range item.(map[string]any) {
				op := raw.(map[string]any)
				if body, ok := op["requestBody"].(map[string]any); ok {
					schema := body["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
					if schema["$ref"] == "#/components/schemas/" {
						schema["$ref"] = "#/components/schemas/EmptyRequest"
					}
				}
			}
		}
	}
	for _, name := range []string{"NameRequest", "ProjectRequest", "Organization", "Workspace", "Team", "Project"} {
		props := b.schemas[name].(map[string]any)["properties"].(map[string]any)
		props["name"].(map[string]any)["maxLength"] = 128
		props["name"].(map[string]any)["x-max-utf8-bytes"] = 128
	}
	for _, name := range []string{"MembershipRequest", "InvitationRequest", "Membership", "Invitation"} {
		b.schemas[name].(map[string]any)["properties"].(map[string]any)["role"].(map[string]any)["enum"] = []string{"member", "owner"}
	}
	for _, name := range []string{"ProjectRequest", "Project"} {
		s := b.schemas[name].(map[string]any)
		s["properties"].(map[string]any)["kind"].(map[string]any)["enum"] = []string{"personal", "team"}
		s["oneOf"] = []any{map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "personal"}}, "required": []string{"ownerId"}, "not": map[string]any{"required": []string{"teamId"}}}, map[string]any{"properties": map[string]any{"kind": map[string]any{"const": "team"}}, "required": []string{"teamId"}, "not": map[string]any{"required": []string{"ownerId"}}}}
	}
	b.schemas["Invitation"].(map[string]any)["properties"].(map[string]any)["state"].(map[string]any)["enum"] = []string{"pending", "consumed", "revoked"}
	b.schemas["InvitationRequest"].(map[string]any)["description"] = "Target must be an exact existing active provider identity. expiresAt must be future and at most seven days from issuance; rechecked after lock waits."
	for _, name := range []string{"AcceptInvitationRequest", "IssuedInvitation"} {
		b.schemas[name].(map[string]any)["properties"].(map[string]any)["secret"] = map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]{43}$", "minLength": 43, "maxLength": 43, "writeOnly": name == "AcceptInvitationRequest", "description": "256-bit random secret exposed once on issuance; only SHA-256 is stored."}
	}
	for _, name := range []string{"OrganizationPage", "WorkspacePage", "TeamPage", "ProjectPage", "MembershipPage", "InvitationPage"} {
		props := b.schemas[name].(map[string]any)["properties"].(map[string]any)
		props["items"].(map[string]any)["maxItems"] = 100
		props["nextCursor"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 2048}
	}
}
