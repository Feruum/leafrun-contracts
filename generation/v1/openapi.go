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
		op := map[string]any{"operationId": id, "responses": responses, "security": []any{map[string]any{"bearer": []string{}}}}
		var parameters []any
		if strings.Contains(path, "{jobId}") {
			parameters = append(parameters, map[string]any{"in": "path", "name": "jobId", "required": true, "schema": identifierSchema(256)})
		}
		if id == "createJob" {
			parameters = append(parameters, map[string]any{"in": "header", "name": "Idempotency-Key", "required": true, "schema": identifierSchema(200), "description": "Scoped to workspace and initiator; retained for seven days. Every replay rechecks current credentials and access."})
			op["description"] = "202 follows durable admission, confirmed activation of the snapshot pin and a durable workflow-start intent. Schema validation is asynchronous. Changed normalized inputs under the same key return 409. JSON normalization preserves numeric lexemes."
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
	add("/v1/jobs/{jobId}", "get", "getJob", "200", nil, reflect.TypeFor[Job]())
	add("/v1/jobs/{jobId}/cancel", "post", "cancelJob", "200", nil, reflect.TypeFor[Job]())
	create := b.schemas["CreateJobRequest"].(map[string]any)
	create["dependentRequired"] = map[string]any{"parameters": []string{"parametersSchemaPath"}, "parametersSchemaPath": []string{"parameters"}}
	for object, fields := range map[string][]string{"Scope": {"workspaceId", "projectId"}, "CreateJobRequest": {"snapshotId", "targetId"}} {
		properties := b.schemas[object].(map[string]any)["properties"].(map[string]any)
		for _, field := range fields {
			properties[field] = identifierSchema(256)
		}
	}
	for _, field := range []string{"entrypoint", "parametersSchemaPath"} {
		create["properties"].(map[string]any)[field] = map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "x-max-utf8-bytes": 4096, "description": "Logical io/fs file path, never a native path or URL. UTF-8 byte length is bounded in addition to JSON Schema character length.", "allOf": []any{map[string]any{"pattern": "^[^/\\\\\\u0000][^\\\\\\u0000]*$"}, map[string]any{"not": map[string]any{"pattern": "(^|/)\\.{1,2}(/|$)"}}, map[string]any{"not": map[string]any{"pattern": "//|/$"}}}}
	}
	return map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Generation API", "version": "1.0.0"}, "paths": paths, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"bearer": map[string]any{"type": "http", "scheme": "bearer"}}}}
}

func identifierSchema(maxBytes int) map[string]any {
	return map[string]any{"type": "string", "minLength": 1, "maxLength": maxBytes, "x-max-utf8-bytes": maxBytes, "allOf": []any{map[string]any{"pattern": "^[^\\u0000\\r\\n]+$"}, map[string]any{"pattern": "^\\S(?:[\\s\\S]*\\S)?$"}}}
}
