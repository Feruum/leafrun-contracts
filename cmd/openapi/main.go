// Command openapi emits the Storage v1 specification from the public DTO schemas.
package main

import (
	"encoding/json"
	"fmt"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	proof "github.com/leafrun/leafrun-contracts/ownerproof/v1"
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
		if t == reflect.TypeFor[v1.PinReleaseState]() {
			s["enum"] = []string{"released", "absent"}
		}
		return s
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int64:
		return map[string]any{"type": "integer", "format": "int64"}
	case reflect.Uint64:
		return map[string]any{"type": "integer", "format": "int64", "minimum": 1, "maximum": identityv1.MaxAuthorityVersion}
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
			if t == reflect.TypeFor[v1.PinReleaseRequest]() || t == reflect.TypeFor[v1.PinReleaseReceipt]() {
				b.schemas[name].(map[string]any)["additionalProperties"] = false
			}
			if t == reflect.TypeFor[identityv1.AuthorityRef]() {
				b.schemas[name].(map[string]any)["additionalProperties"] = false
				properties["kind"].(map[string]any)["enum"] = []string{identityv1.KindKratosSession, identityv1.KindBotKey, identityv1.KindGuestSession, identityv1.KindOAuthToken, identityv1.KindService, identityv1.KindDev}
				properties["id"] = map[string]any{"type": "string", "minLength": 1, "maxLength": identityv1.MaxIdentifierBytes, "x-max-utf8-bytes": identityv1.MaxIdentifierBytes}
			}
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
		if strings.HasPrefix(path, "/v1/") {
			op["security"] = []any{map[string]any{"userBearer": []string{}}, map[string]any{"sessionCookie": []string{}}, map[string]any{"guestCookie": []string{}}}
			op["description"] = "Public current-authority artifact access. Exactly one bearer/human cookie/guest cookie; duplicates, malformed or mixed credentials deny without fallback."
		}
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
	add("post", "/internal/v1/snapshot-bindings/verify", "verifySnapshotBinding", reflect.TypeFor[proof.SnapshotBinding](), reflect.TypeFor[proof.SnapshotBinding](), "200", false, false)
	proofOp := paths["/internal/v1/snapshot-bindings/verify"].(map[string]any)["post"].(map[string]any)
	proofOp["security"] = []any{map[string]any{"evidenceBearer": []string{}}}
	proofOp["x-max-request-bytes"] = proof.Limit
	proofOp["x-max-response-bytes"] = proof.Limit
	proofOp["description"] = "Dedicated Projects owner-evidence credential only. Verify exact committed, ready, unexpired snapshot scope/content binding from catalog facts. No Access callback, source/blob read, pin mutation, or organization ownership assertion."
	add("get", "/internal/v1/snapshots/{snapshotId}", "getSnapshot", nil, reflect.TypeFor[v1.Snapshot](), "200", false, false)
	add("get", "/internal/v1/resources/{resourceId}", "getResource", nil, reflect.TypeFor[v1.Resource](), "200", false, false)
	paths["/internal/v1/resources/{resourceId}"].(map[string]any)["get"].(map[string]any)["description"] = "Current resource-read permission and committed ready lifecycle are checked. This point-in-time metadata response does not retain the resource; acquire a pin before committing its use."
	add("get", "/internal/v1/resources/{resourceId}/content", "resourceContent", nil, nil, "200", false, true)
	add("post", "/internal/v1/pins", "acquirePin", reflect.TypeFor[v1.AcquirePinRequest](), reflect.TypeFor[v1.Pin](), "201", false, false)
	add("get", "/internal/v1/pins/{pinId}", "getPin", nil, reflect.TypeFor[v1.Pin](), "200", false, false)
	add("post", "/internal/v1/pins/{pinId}/activate", "activatePin", nil, reflect.TypeFor[v1.Pin](), "200", false, false)
	add("delete", "/internal/v1/pins/{pinId}", "releasePin", nil, nil, "204", false, false)
	add("post", "/internal/v1/pins/{pinId}/release", "releasePinBound", reflect.TypeFor[v1.PinReleaseRequest](), reflect.TypeFor[v1.PinReleaseReceipt](), "200", false, false)
	paths["/internal/v1/pins/{pinId}/release"].(map[string]any)["post"].(map[string]any)["description"] = "Authorize the authenticated holder on the requested scope before checking the exact object/holder/policy binding. An absent or different binding returns absent without revealing another reference. A 404 is an authorization failure and never confirms absence."
	add("get", "/v1/artifacts/{artifactId}", "getArtifact", nil, reflect.TypeFor[v1.Artifact](), "200", false, false)
	add("get", "/v1/artifacts/{artifactId}/content", "artifactContent", nil, nil, "200", false, true)
	for _, t := range []reflect.Type{reflect.TypeFor[v1.AccessCheck](), reflect.TypeFor[v1.AccessDecision](), reflect.TypeFor[v1.PublishabilityCheck](), reflect.TypeFor[v1.PublishabilityDecision](), reflect.TypeFor[v1.PinOwnerCheck](), reflect.TypeFor[v1.PinOwnerDecision]()} {
		b.schema(t)
	}
	spec := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Storage", "version": "1.0.0", "description": "E03 internal dev API. PDF expires at the original finalize. No presigned download redirects. Owner services remain separate."}, "paths": paths, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"serviceBearer": map[string]any{"type": "http", "scheme": "bearer"}}}}
	schemes := spec["components"].(map[string]any)["securitySchemes"].(map[string]any)
	schemes["userBearer"] = map[string]any{"type": "http", "scheme": "bearer"}
	schemes["sessionCookie"] = map[string]any{"type": "apiKey", "in": "cookie", "name": "ory_kratos_session"}
	schemes["guestCookie"] = map[string]any{"type": "apiKey", "in": "cookie", "name": "leafrun_guest_session"}
	proof.AnnotateSchemas(b.schemas)
	encoder := json.NewEncoder(os.Stdout)
	spec["components"].(map[string]any)["securitySchemes"].(map[string]any)["evidenceBearer"] = map[string]any{"type": "http", "scheme": "bearer", "description": "Dedicated local Projects snapshot evidence verifier credential, distinct from ordinary service and public credentials."}
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(spec); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
