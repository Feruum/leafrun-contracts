package generationv1

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestOpenAPIMatchesDTOsAndGeneratedArtifact(t *testing.T) {
	spec := OpenAPI()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	for _, typ := range []reflect.Type{reflect.TypeFor[CreateJobRequest](), reflect.TypeFor[Job](), reflect.TypeFor[Admission](), reflect.TypeFor[ArtifactBinding]()} {
		properties := schemas[typ.Name()].(map[string]any)["properties"].(map[string]any)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			name := f.Tag.Get("json")
			for j, c := range name {
				if c == ',' {
					name = name[:j]
					break
				}
			}
			if _, ok := properties[name]; !ok {
				t.Errorf("missing %s.%s", typ.Name(), name)
			}
		}
	}
	generated, err := os.ReadFile("../../openapi/generation-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(generated) != string(append(expected, '\n')) {
		t.Fatal("regenerate Generation OpenAPI")
	}
	paths := spec["paths"].(map[string]any)
	create := paths["/v1/jobs"].(map[string]any)["post"].(map[string]any)
	if len(create["parameters"].([]any)) != 1 {
		t.Fatal("missing required Idempotency-Key")
	}
	for _, op := range []map[string]any{create, paths["/v1/jobs/{jobId}"].(map[string]any)["get"].(map[string]any), paths["/v1/jobs/{jobId}/cancel"].(map[string]any)["post"].(map[string]any)} {
		for _, raw := range op["parameters"].([]any) {
			parameter := raw.(map[string]any)
			schema := parameter["schema"].(map[string]any)
			if schema["minLength"] != 1 || schema["x-max-utf8-bytes"] == nil || len(schema["allOf"].([]any)) != 2 {
				t.Fatalf("missing exact identifier bounds: %#v", parameter)
			}
		}
	}
	for _, tc := range []struct {
		object, field string
		max           int
	}{
		{"Scope", "workspaceId", 256}, {"Scope", "projectId", 256}, {"CreateJobRequest", "snapshotId", 256}, {"CreateJobRequest", "targetId", 256},
		{"CreateJobRequest", "entrypoint", 4096}, {"CreateJobRequest", "parametersSchemaPath", 4096},
	} {
		properties := schemas[tc.object].(map[string]any)["properties"].(map[string]any)
		field := properties[tc.field].(map[string]any)
		if field["minLength"] != 1 || field["maxLength"] != tc.max {
			t.Errorf("missing runtime bounds for %s.%s: %#v", tc.object, tc.field, field)
		}
	}
}
