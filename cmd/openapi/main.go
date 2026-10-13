// Command openapi emits the Storage v1 specification from the public DTO schemas.
package main

import (
	"encoding/json"
	"fmt"
	v1 "github.com/leafrun/leafrun-contracts/storage/v1"
	"os"
	"reflect"
	"strings"
	"time"
)

type schemaBuilder struct{ schemas map[string]any }

func (b *schemaBuilder) schema(t reflect.Type) any {
	if t == reflect.TypeFor[time.Time]() {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	if t.Kind() == reflect.Pointer {
		return map[string]any{"anyOf": []any{b.schema(t.Elem()), map[string]any{"type": "null"}}}
	}
	switch t.Kind() {
	case reflect.String:
		s := map[string]any{"type": "string"}
		if t == reflect.TypeFor[v1.ObjectClass]() {
			s["enum"] = []string{"resource", "snapshot", "result", "recovery"}
		}
		if t == reflect.TypeFor[v1.OwnerState]() {
			s["enum"] = []string{"referenced", "absent", "unknown"}
		}
		return s
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}
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
				field := tag[0]
				if field == "" {
					field = f.Name
				}
				properties[field] = b.schema(f.Type)
				if !strings.Contains(f.Tag.Get("json"), "omitempty") {
					required = append(required, field)
				}
			}
			b.schemas[name] = map[string]any{"type": "object", "properties": properties, "required": required}
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	}
	panic("unsupported contract type: " + t.String())
}

func main() {
	b := &schemaBuilder{schemas: map[string]any{}}
	paths := map[string]any{}
	add := func(method, path, id string, input, output reflect.Type, code string, binaryIn, binaryOut bool) {
		responses := map[string]any{}
		success := map[string]any{"description": "Success"}
		if output != nil {
			success["content"] = map[string]any{"application/json": map[string]any{"schema": b.schema(output)}}
		}
		if binaryOut {
			success["content"] = map[string]any{"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}, "application/pdf": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}
		}
		responses[code] = success
		for _, status := range []string{"400", "401", "403", "404", "409", "410", "413", "503"} {
			responses[status] = map[string]any{"description": "Storage error; expiry is disclosed only after authorization", "content": map[string]any{"application/json": map[string]any{"schema": b.schema(reflect.TypeFor[v1.ErrorResponse]())}}}
		}
		op := map[string]any{"operationId": id, "responses": responses, "security": []any{map[string]any{"serviceBearer": []string{}}}}
		parameters := []any{}
		for _, part := range strings.Split(path, "/") {
			if strings.HasPrefix(part, "{") {
				parameters = append(parameters, map[string]any{"name": strings.Trim(part, "{}"), "in": "path", "required": true, "schema": map[string]any{"type": "string"}})
			}
		}
		if binaryIn {
			parameters = append(parameters, map[string]any{"name": "X-Writer-Token", "in": "header", "required": true, "schema": map[string]any{"type": "string"}})
		}
		if len(parameters) > 0 {
			op["parameters"] = parameters
		}
		if input != nil {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": b.schema(input)}}}
		}
		if binaryIn {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}}}}
		}
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path].(map[string]any)[method] = op
	}
	add("post", "/internal/v1/uploads", "createUpload", reflect.TypeFor[v1.CreateUploadRequest](), reflect.TypeFor[v1.Upload](), "201", false, false)
	add("get", "/internal/v1/uploads/{uploadId}", "getUpload", nil, reflect.TypeFor[v1.Upload](), "200", false, false)
	add("put", "/internal/v1/uploads/{uploadId}/content", "putUpload", nil, reflect.TypeFor[v1.Upload](), "200", true, false)
	add("post", "/internal/v1/uploads/{uploadId}/renew", "renewUpload", reflect.TypeFor[v1.RenewUploadRequest](), reflect.TypeFor[v1.Upload](), "200", false, false)
	add("post", "/internal/v1/uploads/{uploadId}/commit", "commitResource", reflect.TypeFor[v1.CommitResourceRequest](), reflect.TypeFor[v1.Resource](), "201", false, false)
	add("post", "/internal/v1/uploads/{uploadId}/finalize", "finalizeArtifact", reflect.TypeFor[v1.FinalizeArtifactRequest](), reflect.TypeFor[v1.Artifact](), "201", false, false)
	add("post", "/internal/v1/snapshots", "createSnapshot", reflect.TypeFor[v1.CreateSnapshotRequest](), reflect.TypeFor[v1.Snapshot](), "201", false, false)
	add("get", "/internal/v1/snapshots/{snapshotId}", "getSnapshot", nil, reflect.TypeFor[v1.Snapshot](), "200", false, false)
	add("get", "/internal/v1/resources/{resourceId}/content", "resourceContent", nil, nil, "200", false, true)
	add("post", "/internal/v1/pins", "acquirePin", reflect.TypeFor[v1.AcquirePinRequest](), reflect.TypeFor[v1.Pin](), "201", false, false)
	add("get", "/internal/v1/pins/{pinId}", "getPin", nil, reflect.TypeFor[v1.Pin](), "200", false, false)
	add("post", "/internal/v1/pins/{pinId}/activate", "activatePin", nil, reflect.TypeFor[v1.Pin](), "200", false, false)
	add("delete", "/internal/v1/pins/{pinId}", "releasePin", nil, nil, "204", false, false)
	add("get", "/v1/artifacts/{artifactId}", "getArtifact", nil, reflect.TypeFor[v1.Artifact](), "200", false, false)
	add("get", "/v1/artifacts/{artifactId}/content", "artifactContent", nil, nil, "200", false, true)
	for _, t := range []reflect.Type{reflect.TypeFor[v1.AccessCheck](), reflect.TypeFor[v1.AccessDecision](), reflect.TypeFor[v1.PublishabilityCheck](), reflect.TypeFor[v1.PublishabilityDecision](), reflect.TypeFor[v1.PinOwnerCheck](), reflect.TypeFor[v1.PinOwnerDecision]()} {
		b.schema(t)
	}
	spec := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Storage", "version": "1.0.0", "description": "E03 internal dev API. PDF expires at the original finalize. No presigned download redirects. Owner services remain separate."}, "paths": paths, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"serviceBearer": map[string]any{"type": "http", "scheme": "bearer"}}}}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(spec); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
