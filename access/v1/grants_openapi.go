package accessv1

import (
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"reflect"
	"strings"
)

func grantsOpenAPI(b *schemaBuilder, paths map[string]any) {
	for _, r := range []struct {
		method, path, id string
		in               reflect.Type
	}{
		{"post", "/v1/workspaces/{workspace}/projects/{project}/grants", "createGrant", reflect.TypeFor[CreateGrantRequest]()},
		{"post", "/v1/grants/{grant}/rotate", "rotateGrant", reflect.TypeFor[RotateGrantRequest]()},
		{"delete", "/v1/grants/{grant}", "revokeGrant", reflect.TypeFor[RevokeGrantRequest]()},
	} {
		params := []any{}
		for _, part := range strings.Split(r.path, "/") {
			if strings.HasPrefix(part, "{") {
				params = append(params, map[string]any{"name": strings.Trim(part, "{}"), "in": "path", "required": true, "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "x-max-utf8-bytes": 256}})
			}
		}
		responses := map[string]any{"200": map[string]any{"description": "Committed result. First issuance/rotation discloses random link secret once; retry is metadata-only.", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[GrantResult]())}}}}
		for _, status := range []string{"400", "401", "403", "409", "410", "503"} {
			responses[status] = map[string]any{"description": "Invalid, unauthenticated, denied, stale/changed-input conflict, or dependency uncertainty. No secret-bearing errors."}
		}
		paths[r.path] = map[string]any{r.method: map[string]any{"operationId": r.id, "description": "Exact registered tenant/workspace/project. Current owner and credential ceiling checked under locks. Role at most editor; no owner/admin/credential/grant delegation. Rotation only narrows immutable issued ceiling. Finite expiry, monotonically increasing version; hash-only link secret.", "parameters": params, "security": []any{map[string]any{"userBearer": []string{}}, map[string]any{"sessionCookie": []string{}}}, "requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(r.in)}}}, "responses": responses}}
	}
	createOp := paths["/v1/workspaces/{workspace}/projects/{project}/grants"].(map[string]any)["post"].(map[string]any)
	createOp["description"] = createOp["description"].(string) + " New creates require a canonical UUIDv7 operationKey with immutable issuance timestamp within the fixed v1 30-day retry window (at most one minute future clock skew). Use NewGrantOperationKey and retain the original key/body. Configurable metadata retention cannot extend this immutable key validity. An expired key returns 410 even after metadata cleanup. Arbitrary legacy keys only replay matching retained receipts until their fixed 30-day expiry; they never admit a new create."
	createOp["responses"].(map[string]any)["410"] = map[string]any{"description": "Operation retry window expired; the same key can never issue another secret."}
	guestBrowserOpenAPI(b, paths)
	for _, typ := range []reflect.Type{reflect.TypeFor[ExchangeGrantRequest](), reflect.TypeFor[GuestLogoutRequest]()} {
		b.schema(typ)
	}
	for _, name := range []string{"CreateGrantRequest", "RotateGrantRequest"} {
		schema := b.schemas[name].(map[string]any)
		props := schema["properties"].(map[string]any)
		props["role"] = map[string]any{"type": "string", "enum": []string{"", "reader", "commenter", "editor"}}
		props["operations"] = map[string]any{"type": "array", "minItems": 0, "maxItems": 12, "uniqueItems": true, "items": map[string]any{"type": "string", "enum": []string{"project.read", "comment.create", "project.write", "generation.cancel", "generation.create", "generation.execute", "generation.publish", "generation.read", "source.read", "resource.read", "artifact.read", "download"}}}
		schema["anyOf"] = []any{map[string]any{"required": []string{"role"}, "properties": map[string]any{"role": map[string]any{"enum": []string{"reader", "commenter", "editor"}}}}, map[string]any{"required": []string{"operations"}, "properties": map[string]any{"operations": map[string]any{"minItems": 1}}}}
	}
	for _, name := range []string{"RotateGrantRequest", "RevokeGrantRequest"} {
		b.schemas[name].(map[string]any)["properties"].(map[string]any)["expectedVersion"].(map[string]any)["maximum"] = identityv1.MaxAuthorityVersion - 1
	}
	result := b.schemas["GrantResult"].(map[string]any)
	props := result["properties"].(map[string]any)
	props["disclosure"] = map[string]any{"type": "string", "enum": []string{"first", "unavailable"}}
	props["secret"] = map[string]any{"type": "string", "minLength": 52, "maxLength": 52, "pattern": "^lr_grant_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$"}
	result["oneOf"] = []any{map[string]any{"properties": map[string]any{"disclosure": map[string]any{"const": "first"}}, "required": []string{"secret"}}, map[string]any{"properties": map[string]any{"disclosure": map[string]any{"const": "unavailable"}}, "not": map[string]any{"required": []string{"secret"}}}}
	grant := b.schemas["Grant"].(map[string]any)["properties"].(map[string]any)
	grant["status"] = map[string]any{"type": "string", "enum": []string{"active", "revoked"}}
	grant["role"] = map[string]any{"type": "string", "enum": []string{"", "reader", "commenter", "editor"}}
	exactScope := map[string]any{"required": []string{"organizationId", "workspaceId", "projectId"}}
	grant["scope"] = map[string]any{"allOf": []any{grant["scope"], exactScope}}
	ceilingOps := map[string]any{"type": "array", "minItems": 1, "maxItems": 12, "uniqueItems": true, "items": map[string]any{"type": "string", "enum": []string{"project.read", "comment.create", "project.write", "generation.cancel", "generation.create", "generation.execute", "generation.publish", "generation.read", "source.read", "resource.read", "artifact.read", "download"}}}
	ceilingFields := map[string]any{"properties": map[string]any{"scope": exactScope, "operations": ceilingOps}}
	grant["ceiling"] = map[string]any{"allOf": []any{grant["ceiling"], ceilingFields}}
	result["oneOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)["grant"] = map[string]any{"properties": map[string]any{"status": map[string]any{"const": "active"}}}
	exchange := b.schemas["ExchangeGrantRequest"].(map[string]any)["properties"].(map[string]any)
	exchange["secret"] = map[string]any{"type": "string", "minLength": 52, "maxLength": 52, "pattern": "^lr_grant_[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$", "writeOnly": true}
	exchange["csrf"] = map[string]any{"type": "string", "minLength": 1, "maxLength": MaxCredentialBytes, "writeOnly": true}
	b.schemas["GuestLogoutRequest"].(map[string]any)["properties"].(map[string]any)["csrf"] = exchange["csrf"]
}

func guestBrowserOpenAPI(b *schemaBuilder, paths map[string]any) {
	paths["/v1/guest/challenge"] = map[string]any{"get": map[string]any{"operationId": "guestChallenge", "description": "Finite one-use same-origin action/browser/selected-session-bound challenge. Exchange supports anonymous activation; logout has only a narrow guest cleanup exception when both cookies are present. No ordinary-domain cookie precedence.", "parameters": []any{map[string]any{"name": "action", "in": "query", "required": true, "schema": map[string]any{"type": "string", "enum": []string{"exchange", "logout"}}}}, "responses": map[string]any{"200": map[string]any{"description": "Challenge with private browser-state Set-Cookie.", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "additionalProperties": false, "required": []string{"csrf"}, "properties": map[string]any{"csrf": map[string]any{"type": "string", "minLength": 1, "maxLength": MaxCredentialBytes}}}}}}}}}
	paths["/v1/grants/exchange"] = map[string]any{"post": map[string]any{"operationId": "exchangeGrant", "description": "Fixed same-origin protected activation. Secret only in bounded JSON body. Refuses any presented human cookie without changing human session. Existing guest replacement requires explicit replace and selected-session-bound CSRF. Installs HttpOnly/SameSite/finite-lifetime guest cookie, Secure outside explicit loopback; never returns its raw credential.", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[ExchangeGrantRequest]())}}}, "responses": map[string]any{"200": map[string]any{"description": "Separate guest session installed via Set-Cookie.", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[ResolveResponse]())}}}, "400": map[string]any{"description": "Invalid bounded input."}, "401": map[string]any{"description": "Invalid current grant/session."}, "403": map[string]any{"description": "Origin, CSRF, human coexistence or activation denied."}, "503": map[string]any{"description": "Dependency uncertainty."}}}}
	paths["/v1/guest/logout"] = map[string]any{"post": map[string]any{"operationId": "logoutGuest", "description": "Same-origin finite one-use selected-session-bound CSRF. Revokes only this guest session and clears only guest cookie. Reusable grant, other guest sessions and human session survive. Narrow explicit both-cookie cleanup cannot authorize ordinary domain operations.", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[GuestLogoutRequest]())}}}, "responses": map[string]any{"204": map[string]any{"description": "Guest authority revoked and cookie cleared; empty body."}, "401": map[string]any{"description": "Guest session required."}, "403": map[string]any{"description": "Origin/CSRF/selected-session binding denied."}, "503": map[string]any{"description": "Dependency uncertainty."}}}}
}
