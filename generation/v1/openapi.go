package generationv1

import (
	"encoding/json"
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
		return map[string]any{"type": "object", "additionalProperties": true}
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{b.schema(t.Elem()), map[string]any{"type": "null"}}}
	}
	switch t.Kind() {
	case reflect.String:
		schema := map[string]any{"type": "string"}
		if t == reflect.TypeFor[Status]() {
			schema["enum"] = []string{string(Queued), string(Preparing), string(Rendering), string(Publishing), string(Cancelling), string(Succeeded), string(Failed), string(Canceled), string(Expired)}
		}
		return schema
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
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
		panic("unsupported Generation DTO: " + t.String())
	}
}

// OpenAPI generates the public HTTP contract from the versioned DTOs. Wire
// framing and internal owner checks are documented separately; Storage v1 owns
// the shared Access/Publishable/PinOwner request and response schemas.
func OpenAPI() map[string]any {
	b := &schemaBuilder{schemas: map[string]any{}}
	paths := map[string]any{}
	add := func(path, method, id, code string, input, output reflect.Type) {
		responses := map[string]any{code: map[string]any{"description": "Success", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(output)}}}}
		for _, status := range []string{"400", "401", "403", "404", "409", "413", "429", "503"} {
			responses[status] = map[string]any{"description": "Generation error; access is checked before disclosure", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[ErrorResponse]())}}}
		}
		if id == "createRender" {
			responses["422"] = map[string]any{"description": "Schema, file or profile validation failed, or the selected target kind is unsupported", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[ErrorResponse]())}}}
		}
		op := map[string]any{"operationId": id, "responses": responses, "security": []any{map[string]any{"bearer": []string{}}}}
		if strings.HasPrefix(path, "/v1/") {
			op["security"] = []any{map[string]any{"bearer": []string{}}, map[string]any{"sessionCookie": []string{}}, map[string]any{"guestCookie": []string{}}}
			op["x-public-credential-selection"] = "Exactly one bearer/human cookie/guest cookie. Duplicate, malformed, empty, oversized or ambiguous candidates deny before resolution. Unsafe cookie requests require explicit configured Origin."
		}
		var parameters []any
		if strings.Contains(path, "{jobId}") {
			parameters = append(parameters, map[string]any{"in": "path", "name": "jobId", "required": true, "schema": identifierSchema(256)})
		}
		if id == "createJob" || id == "createRender" {
			parameters = append(parameters, map[string]any{"in": "header", "name": "Idempotency-Key", "required": true, "schema": identifierSchema(200), "description": "Scoped to workspace and initiator; retained for seven days. Every replay rechecks current credentials and access."})
			op["description"] = "202 follows durable admission, confirmed activation of the snapshot pin and a durable workflow-start intent. Schema validation is asynchronous. Changed normalized inputs under the same key return 409. JSON normalization preserves numeric lexemes."
		}
		if id == "createRender" {
			op["description"] = "Resolve an immutable published release or registered working target through Projects. Current access is checked before replay; exact recorded replays do not resolve a newer revision. New admission follows schema/default/file/profile preparation and activation of every input and source pin. Bundle execution is rejected before admission. Cache policy records intent; reuse belongs to E15."
		}
		if id == "getJob" {
			op["description"] = "Current access is required. Parameters and raw/source diagnostics are omitted without source-read permission. A committed success remains historical after PDF expiry."
		}
		if id == "cancelJob" {
			op["description"] = "Idempotent cancellation. Terminal success and cancellation serialize; a later cancellation does not undo success."
		}
		if len(parameters) > 0 {
			op["parameters"] = parameters
		}
		if input != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(input)}}, "x-max-bytes": 1 << 20}
		}
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path].(map[string]any)[method] = op
	}
	add("/v1/jobs", "post", "createJob", "202", reflect.TypeFor[CreateJobRequest](), reflect.TypeFor[Admission]())
	add("/v1/renders", "post", "createRender", "202", reflect.TypeFor[RenderSelectionRequest](), reflect.TypeFor[Admission]())
	add("/v1/jobs/{jobId}", "get", "getJob", "200", nil, reflect.TypeFor[Job]())
	add("/v1/jobs/{jobId}/cancel", "post", "cancelJob", "200", nil, reflect.TypeFor[Job]())
	create := b.schemas["CreateJobRequest"].(map[string]any)
	create["dependentRequired"] = map[string]any{"parameters": []string{"parametersSchemaPath"}, "parametersSchemaPath": []string{"parameters"}}
	selection := b.schemas["RenderSelectionRequest"].(map[string]any)
	selection["required"] = []string{"scope"}
	selection["oneOf"] = []any{
		map[string]any{"required": []string{"releaseId"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"snapshotId"}}, map[string]any{"required": []string{"targetId"}}}}},
		map[string]any{"required": []string{"snapshotId", "targetId"}, "not": map[string]any{"required": []string{"releaseId"}}},
	}
	props := selection["properties"].(map[string]any)
	for _, field := range []string{"releaseId", "snapshotId", "targetId", "profile"} {
		props[field] = identifierSchema(256)
	}
	props["cachePolicy"] = map[string]any{"type": "string", "enum": []string{"use", "refresh"}, "default": "use"}
	props["parameters"].(map[string]any)["default"] = map[string]any{}
	props["fileRefs"].(map[string]any)["maxProperties"] = 64
	props["fileRefs"].(map[string]any)["default"] = map[string]any{}
	props["fileRefs"].(map[string]any)["propertyNames"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "pattern": "^/([^~]|~[01])*$", "x-max-utf8-bytes": 4096}
	fileProps := b.schemas["FileReference"].(map[string]any)["properties"].(map[string]any)
	fileProps["resourceId"] = identifierSchema(256)
	fileProps["filename"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 255, "x-max-utf8-bytes": 255, "description": "A nonempty base filename without separators, colon, control characters or surrounding whitespace. If present, its final extension is a dot followed by 1–16 ASCII letters/digits. Core generates the file path from SHA-256 and this exact extension.", "allOf": []any{
		map[string]any{"pattern": "^[^/\\\\:\\u0000-\\u001f\\u007f-\\u009f]+$"},
		map[string]any{"not": map[string]any{"enum": []string{".", ".."}}},
		map[string]any{"pattern": "^\\S(?:[\\s\\S]*\\S)?$"},
		map[string]any{"anyOf": []any{map[string]any{"not": map[string]any{"pattern": "\\."}}, map[string]any{"pattern": "\\.[A-Za-z0-9]{1,16}$"}}},
	}}
	for object, fields := range map[string][]string{"Scope": {"workspaceId", "projectId"}, "CreateJobRequest": {"snapshotId", "targetId"}} {
		properties := b.schemas[object].(map[string]any)["properties"].(map[string]any)
		for _, field := range fields {
			properties[field] = identifierSchema(256)
		}
	}
	for _, field := range []string{"entrypoint", "parametersSchemaPath"} {
		create["properties"].(map[string]any)[field] = map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "x-max-utf8-bytes": 4096, "description": "Logical io/fs file path, never a native path or URL. UTF-8 byte length is bounded in addition to JSON Schema character length.", "allOf": []any{map[string]any{"pattern": "^[^/\\\\\\u0000][^\\\\\\u0000]*$"}, map[string]any{"not": map[string]any{"pattern": "(^|/)\\.{1,2}(/|$)"}}, map[string]any{"not": map[string]any{"pattern": "//|/$"}}}}
	}
	return map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Generation API", "version": "1.0.0"}, "paths": paths, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"sessionCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "ory_kratos_session", "description": "Deployment-configured human cookie."}, "guestCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "leafrun_guest_session", "description": "Deployment-configured Access guest cookie."}, "bearer": map[string]any{"type": "http", "scheme": "bearer"}}}}
}

func identifierSchema(maxBytes int) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": maxBytes, "x-max-utf8-bytes": maxBytes, "allOf": []any{map[string]any{"pattern": "^[^\\u0000\\r\\n]+$"}, map[string]any{"pattern": "^\\S(?:[\\s\\S]*\\S)?$"}}}
}
