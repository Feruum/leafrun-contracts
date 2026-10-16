package projectsv1

import (
	"encoding/json"
	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	proof "github.com/leafrun/leafrun-contracts/ownerproof/v1"
	"reflect"
	"strings"
	"time"
)

type schemaBuilder struct{ schemas map[string]any }

func (b *schemaBuilder) schema(t reflect.Type) any {
	if t == reflect.TypeFor[time.Time]() {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	if t == reflect.TypeFor[json.RawMessage]() {
		return map[string]any{"anyOf": []any{map[string]any{"type": "object"}, map[string]any{"type": "boolean"}}}
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{b.schema(t.Elem()), map[string]any{"type": "null"}}}
	}
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}
	case reflect.Uint64:
		return map[string]any{"type": "integer", "format": "int64", "minimum": 1, "maximum": identityv1.MaxAuthorityVersion}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": b.schema(t.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": b.schema(t.Elem())}
	case reflect.Struct:
		name := t.Name()
		if _, ok := b.schemas[name]; !ok {
			b.schemas[name] = map[string]any{}
			properties := map[string]any{}
			required := []string{}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				tag := strings.Split(f.Tag.Get("json"), ",")
				if tag[0] == "-" {
					continue
				}
				properties[tag[0]] = b.schema(f.Type)
				if len(tag) == 1 {
					required = append(required, tag[0])
				}
			}
			b.schemas[name] = map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	default:
		panic("unsupported Projects DTO: " + t.String())
	}
}

func OpenAPI() map[string]any {
	b := &schemaBuilder{schemas: map[string]any{}}
	paths := map[string]any{}
	add := func(path, method, id, code string, in, out reflect.Type) {
		responses := map[string]any{}
		success := map[string]any{"description": "Success"}
		if out != nil {
			success["content"] = map[string]any{"application/json": map[string]any{"schema": b.schema(out)}}
		}
		responses[code] = success
		for _, status := range []string{"400", "401", "403", "404", "409", "410", "413", "422", "503"} {
			responses[status] = map[string]any{"description": "Current access is checked before disclosure. Dependency uncertainty fails closed.", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[ErrorResponse]())}}}
		}
		op := map[string]any{"operationId": id, "responses": responses, "security": []any{map[string]any{"bearer": []string{}}}}
		if strings.HasPrefix(path, "/v1/") {
			op["security"] = []any{map[string]any{"bearer": []string{}}, map[string]any{"sessionCookie": []string{}}, map[string]any{"guestCookie": []string{}}}
			op["x-public-credential-selection"] = "Exactly one bearer/human cookie/guest cookie. Duplicate, malformed, empty, oversized or ambiguous candidates deny before resolution. Unsafe cookie requests require explicit configured Origin."
		}
		var parameters []any
		for _, part := range strings.Split(path, "/") {
			if strings.HasPrefix(part, "{") {
				parameters = append(parameters, map[string]any{"in": "path", "name": strings.Trim(part, "{}"), "required": true, "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": 256, "x-max-utf8-bytes": 256}})
			}
		}
		if (method == "get" && strings.Contains(path, "revisions")) || (method == "get" && path == "/v1/releases") {
			for _, name := range []string{"workspaceId", "projectId"} {
				parameters = append(parameters, map[string]any{"in": "query", "name": name, "required": true, "schema": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}})
			}
		}
		if len(parameters) > 0 {
			op["parameters"] = parameters
		}
		if in != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(in)}}}
		}
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path].(map[string]any)[method] = op
	}
	add("/v1/revisions", "post", "registerRevision", "201", reflect.TypeFor[RegisterRevisionRequest](), reflect.TypeFor[Revision]())
	add("/v1/revisions/{revision}", "get", "getRevision", "200", nil, reflect.TypeFor[Revision]())
	add("/v1/revisions/{revision}", "delete", "deleteRevision", "204", reflect.TypeFor[DeleteRevisionRequest](), nil)
	add("/v1/releases", "post", "createRelease", "202", reflect.TypeFor[CreateReleaseRequest](), reflect.TypeFor[Release]())
	add("/v1/releases", "get", "catalog", "200", nil, reflect.TypeFor[Catalog]())
	add("/v1/releases/{releaseId}", "get", "getRelease", "200", nil, reflect.TypeFor[Release]())
	add("/v1/releases/{releaseId}", "delete", "deleteRelease", "204", reflect.TypeFor[DeleteReleaseRequest](), nil)
	add("/v1/releases/{releaseId}/publication", "post", "setPublication", "200", reflect.TypeFor[PublicationRequest](), reflect.TypeFor[Release]())
	add("/v1/releases/{releaseId}/form", "get", "releaseForm", "200", nil, reflect.TypeFor[ReleaseForm]())
	add("/internal/v1/resolve", "post", "resolveExecution", "200", reflect.TypeFor[ResolveRequest](), reflect.TypeFor[ResolvedExecution]())
	add("/internal/v1/checks/release", "post", "currentRelease", "200", reflect.TypeFor[CurrentReleaseCheck](), reflect.TypeFor[CurrentReleaseDecision]())
	add("/internal/v1/scopes/verify", "post", "verifyLegacyScope", "200", reflect.TypeFor[proof.ScopeRequest](), reflect.TypeFor[proof.RevisionBinding]())
	proofOp := paths["/internal/v1/scopes/verify"].(map[string]any)["post"].(map[string]any)
	proofOp["security"] = []any{map[string]any{"evidenceBearer": []string{}}}
	proofOp["x-max-request-bytes"] = proof.Limit
	proofOp["x-max-response-bytes"] = proof.Limit
	proofOp["description"] = "Dedicated Access owner-evidence credential only. Exact live revision lookup and independent committed Storage snapshot proof; no public user authorization callback, source bytes, or organization ownership assertion."
	paths["/v1/releases"].(map[string]any)["post"].(map[string]any)["description"] = "Persist an immutable technically validated release and durable Storage release-pin acquisition intent. Pending releases become usable only after confirmed activation. No Typst invocation or successful preview is required. Changed inputs under an existing version or idempotency key conflict."
	paths["/internal/v1/resolve"].(map[string]any)["post"].(map[string]any)["description"] = "Fixed Generation service credential required. Actor is asserted by authenticated server-side Generation code. Verify current caller rights and publication; bind response to actor and selection fingerprint. Prepare defaults/files once through shared Core. Reject bundle execution before Generation admission."
	paths["/v1/releases/{releaseId}"].(map[string]any)["delete"].(map[string]any)["description"] = "Requires an unpublished release and matching publication version. Tombstone and durably release its Storage reference. Unpublish alone preserves rollback identity and retention."
	// Carry the same public selector contract into the privileged owner DTO.
	generationSchemas := generationv1.OpenAPI()["components"].(map[string]any)["schemas"].(map[string]any)
	for _, name := range []string{"RenderSelectionRequest", "FileReference", "Scope"} {
		b.schemas[name] = generationSchemas[name]
	}
	b.schemas["PreparedInput"].(map[string]any)["properties"].(map[string]any)["json"] = map[string]any{"type": "object", "x-max-bytes": 1 << 20}
	doc := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Projects API", "version": "1.0.0"}, "paths": paths, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"sessionCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "ory_kratos_session", "description": "Deployment-configured human cookie."}, "guestCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "leafrun_guest_session", "description": "Deployment-configured Access guest cookie."}, "bearer": map[string]any{"type": "http", "scheme": "bearer"}}}}
	doc["components"].(map[string]any)["securitySchemes"].(map[string]any)["evidenceBearer"] = map[string]any{"type": "http", "scheme": "bearer", "description": "Dedicated Access legacy scope evidence verifier credential, distinct from all public and ordinary service credentials."}
	proof.AnnotateSchemas(b.schemas)
	return doc
}
