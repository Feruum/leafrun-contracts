package accessv1

import (
	"reflect"
	"strings"
	"time"

	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	proof "github.com/leafrun/leafrun-contracts/ownerproof/v1"
)

type schemaBuilder struct{ schemas map[string]any }

// These are literal Unicode ranges, shared by Go RE2 and ECMA-262 rather than
// engine-specific Unicode property escapes. C0/C1 are Unicode control characters;
// the remaining ranges are Unicode whitespace recognized by strings.TrimSpace.
const identifierControls = "\u0000-\u001f\u007f-\u009f"
const identifierWhitespace = "\u0020\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000"
const identifierBoundary = identifierControls + identifierWhitespace

func (b *schemaBuilder) schema(t reflect.Type) any {
	if t == reflect.TypeFor[time.Time]() {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{b.schema(t.Elem()), map[string]any{"type": "null"}}}
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{
			"type": "string", "minLength": 1, "maxLength": 256, "x-max-utf8-bytes": 256,
			"pattern": "^[^" + identifierBoundary + "](?:[^" + identifierControls + "]*[^" + identifierBoundary + "])?$",
			// Explicit exclusion keeps final newline/line-separator values invalid
			// on ECMA-262 engines where $ also matches before a final terminator.
			"not":         map[string]any{"pattern": "[" + identifierControls + "]|^[" + identifierWhitespace + "]|[" + identifierWhitespace + "]$"},
			"description": "Nonempty valid UTF-8 identifier, at most 256 UTF-8 bytes, without leading/trailing Unicode whitespace or any Unicode control characters. maxLength counts Unicode characters; x-max-utf8-bytes states the separate byte limit.",
		}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int:
		return map[string]any{"type": "integer"}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": b.schema(t.Elem())}
	case reflect.Uint64:
		return map[string]any{"type": "integer", "format": "int64", "minimum": 1, "maximum": identityv1.MaxAuthorityVersion}
	case reflect.Struct:
		name := t.Name()
		for typ, stable := range map[reflect.Type]string{reflect.TypeFor[Page[Organization]](): "OrganizationPage", reflect.TypeFor[Page[Workspace]](): "WorkspacePage", reflect.TypeFor[Page[Team]](): "TeamPage", reflect.TypeFor[Page[Project]](): "ProjectPage", reflect.TypeFor[Page[Membership]](): "MembershipPage", reflect.TypeFor[Page[Invitation]](): "InvitationPage"} {
			if t == typ {
				name = stable
			}
		}
		if _, ok := b.schemas[name]; !ok {
			b.schemas[name] = map[string]any{}
			properties := map[string]any{}
			required := []string{}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				tag := strings.Split(f.Tag.Get("json"), ",")
				properties[tag[0]] = b.schema(f.Type)
				if len(tag) == 1 {
					required = append(required, tag[0])
				}
			}
			b.schemas[name] = map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	default:
		panic("unsupported Access DTO: " + t.String())
	}
}

// NewOpenAPI emits the same closed DTO shapes used by the strict client. It
// describes syntax, not permission policy or the authoritative hierarchy registry.
func NewOpenAPI() map[string]any {
	b := &schemaBuilder{schemas: map[string]any{}}
	paths := map[string]any{}
	add := func(path, id string, in, out reflect.Type) {
		responses := map[string]any{"200": map[string]any{"description": "Success", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(out)}}}}
		for _, status := range []string{"400", "401", "403", "503"} {
			responses[status] = map[string]any{"description": "Invalid request, invalid authentication, denied, or dependency uncertainty. Error bodies never echo credentials."}
		}
		paths[path] = map[string]any{"post": map[string]any{"operationId": id, "security": []any{map[string]any{"serviceBearer": []string{}}}, "requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(in)}}}, "responses": responses}}
	}
	add("/internal/v1/auth/resolve", "resolveIdentity", reflect.TypeFor[ResolveRequest](), reflect.TypeFor[ResolveResponse]())
	add("/internal/v1/checks/access", "checkAccess", reflect.TypeFor[CheckRequest](), reflect.TypeFor[CheckResponse]())
	add("/internal/v1/scopes/import", "importLegacyScope", reflect.TypeFor[proof.ImportRequest](), reflect.TypeFor[proof.ImportResult]())
	importOp := paths["/internal/v1/scopes/import"].(map[string]any)["post"].(map[string]any)
	importOp["security"] = []any{map[string]any{"operatorBearer": []string{}}}
	importOp["x-max-request-bytes"] = proof.Limit
	importOp["x-max-response-bytes"] = proof.Limit
	importOp["description"] = "Dedicated operator credential only; public organization owners cannot claim existing IDs. Verify exact live Projects revision and committed Storage snapshot through fixed owner credentials. Organization assignment is an explicit operator assertion. Immutable exact text scope and idempotent claim; concurrent different-organization claims conflict."
	importOp["responses"].(map[string]any)["404"] = map[string]any{"description": "Owner evidence or destination unavailable"}
	importOp["responses"].(map[string]any)["409"] = map[string]any{"description": "Immutable claim or changed receipt input conflict"}
	paths["/v1/session"] = map[string]any{"get": map[string]any{"operationId": "currentSession", "description": "Derives the current identity from exactly one explicit public caller credential. Fixed service credentials are rejected. Cookie name is deployment-configured; default shown below.", "security": []any{map[string]any{"userBearer": []string{}}, map[string]any{"sessionCookie": []string{}}}, "responses": map[string]any{"200": map[string]any{"description": "Verified current session", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[ResolveResponse]())}}}, "401": map[string]any{"description": "Invalid authentication"}, "503": map[string]any{"description": "Authentication dependency unavailable"}}}}
	b.schemas["CheckRequest"].(map[string]any)["properties"].(map[string]any)["operation"].(map[string]any)["pattern"] = operationPattern.String()
	resolve := b.schemas["ResolveRequest"].(map[string]any)
	for _, field := range []string{"bearer", "sessionCookie"} {
		resolve["properties"].(map[string]any)[field] = map[string]any{"type": "string", "minLength": 1, "maxLength": MaxCredentialBytes, "x-max-utf8-bytes": MaxCredentialBytes, "writeOnly": true, "description": "Opaque credential VALUE; no whitespace, controls, commas or semicolons. Session cookie is only one configured human/guest cookie value, not a Cookie header."}
	}
	resolve["properties"].(map[string]any)["sessionCookieKind"] = map[string]any{"type": "string", "enum": []string{"kratos", "guest"}, "description": "Server-derived from the validated configured cookie name. Omission preserves legacy Kratos; only valid on the sessionCookie branch. Never a provider URL or actor selector."}
	resolve["dependentRequired"] = map[string]any{"sessionCookieKind": []string{"sessionCookie"}}
	resolve["oneOf"] = []any{map[string]any{"required": []string{"bearer"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"sessionCookie"}}, map[string]any{"required": []string{"sessionCookieKind"}}}}}, map[string]any{"required": []string{"sessionCookie"}, "not": map[string]any{"required": []string{"bearer"}}}}
	response := b.schemas["ResolveResponse"].(map[string]any)
	response["description"] = "Finite future expiresAt is mandatory for kratos_session, guest_session and oauth_token. Bot/service/dev may omit expiry; any supplied finite expiry must be nonzero and future. No authentication cache; each action checks current authority state."
	response["anyOf"] = []any{
		map[string]any{"required": []string{"expiresAt"}, "properties": map[string]any{"expiresAt": map[string]any{"type": "string", "format": "date-time"}}},
		map[string]any{"properties": map[string]any{"identity": map[string]any{"properties": map[string]any{"authority": map[string]any{"properties": map[string]any{"kind": map[string]any{"enum": []string{identityv1.KindBotKey, identityv1.KindService, identityv1.KindDev}}}}}}}},
	}
	scope := b.schemas["PermissionScope"].(map[string]any)
	shape := func(required, absent []string) any {
		s := map[string]any{"required": required}
		forbidden := []any{}
		for _, field := range absent {
			forbidden = append(forbidden, map[string]any{"required": []string{field}})
		}
		if len(forbidden) > 0 {
			s["not"] = map[string]any{"anyOf": forbidden}
		}
		return s
	}
	scope["oneOf"] = []any{shape([]string{"organizationId"}, []string{"workspaceId", "projectId"}), shape([]string{"organizationId", "workspaceId"}, []string{"projectId"}), shape([]string{"organizationId", "workspaceId", "projectId"}, nil), shape([]string{"workspaceId", "projectId"}, []string{"organizationId"})}
	scope["description"] = "Canonical organization, organization/workspace, organization/workspace/project; legacy exact workspace/project pair permits omitted organization only. Access derives the organization from its unique registered exact pair and compares all supplied hierarchy IDs to its registry."
	b.schemas["AuthorityRef"].(map[string]any)["properties"].(map[string]any)["kind"].(map[string]any)["enum"] = []string{identityv1.KindKratosSession, identityv1.KindBotKey, identityv1.KindGuestSession, identityv1.KindOAuthToken, identityv1.KindService, identityv1.KindDev}
	hierarchyOpenAPI(b, paths)
	botsOpenAPI(b, paths)
	oauthOpenAPI(b, paths)
	grantsOpenAPI(b, paths)
	check := b.schemas["CheckRequest"].(map[string]any)
	check["description"] = "Only the closed operation vocabulary is accepted. Nonservice actors require exact current bound authority. Service caller credential and actor remain separate. Technical health uses an empty scope and an exact configured service/action allowlist, never a fake tenant."
	check["properties"].(map[string]any)["scope"] = map[string]any{"anyOf": []any{map[string]any{"$ref": "#/components/schemas/PermissionScope"}, map[string]any{"type": "object", "maxProperties": 0}}}
	check["allOf"] = []any{map[string]any{"if": map[string]any{"properties": map[string]any{"scope": map[string]any{"maxProperties": 0}}}, "then": map[string]any{"properties": map[string]any{"operation": map[string]any{"const": "health"}, "principal": map[string]any{"properties": map[string]any{"domain": map[string]any{"const": "service"}}}}}}}
	check["allOf"] = append(check["allOf"].([]any), map[string]any{"if": map[string]any{"properties": map[string]any{"operation": map[string]any{"const": "generation.cleanup"}}}, "then": map[string]any{"properties": map[string]any{"principal": map[string]any{"properties": map[string]any{"domain": map[string]any{"const": "service"}}}, "scope": map[string]any{"required": []string{"workspaceId", "projectId"}}}, "not": map[string]any{"required": []string{"authority"}}}})
	check["description"] = check["description"].(string) + " generation.cleanup is service-only, exact configured service-caller/actor/action allowlist, workspace/project registry binding required, no caller authority or human/bot/guest delegation. It authorizes bounded cleanup only and cannot publish a successful result."
	doc := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Access API", "version": "1.0.0"}, "paths": paths, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"serviceBearer": map[string]any{"type": "http", "scheme": "bearer", "description": "Fixed service caller credential, separate from actor authority and public caller credentials."}, "userBearer": map[string]any{"type": "http", "scheme": "bearer", "description": "User session token, separate from fixed internal credentials."}, "sessionCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "ory_kratos_session", "description": "Only the deployment-configured session cookie; mutually exclusive with userBearer."}}}}
	doc["components"].(map[string]any)["securitySchemes"].(map[string]any)["operatorBearer"] = map[string]any{"type": "http", "scheme": "bearer", "description": "Dedicated trusted legacy import operator credential, distinct from all public and ordinary service credentials."}
	schemes := doc["components"].(map[string]any)["securitySchemes"].(map[string]any)
	schemes["guestCookie"] = map[string]any{"type": "apiKey", "in": "cookie", "name": "leafrun_guest_session", "description": "Deployment-configured Access guest cookie. Exactly one of bearer, human cookie, guest cookie; ambiguity/duplicates/malformed values deny without fallback."}
	session := paths["/v1/session"].(map[string]any)["get"].(map[string]any)
	session["security"] = append(session["security"].([]any), map[string]any{"guestCookie": []string{}})
	proof.AnnotateSchemas(b.schemas)
	return doc

}
