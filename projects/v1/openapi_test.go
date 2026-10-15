package projectsv1

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
)

func TestProjectsOpenAPITracksDTOsAndGeneratedBytes(t *testing.T) {
	spec := OpenAPI()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	if _, exists := schemas[""]; exists {
		t.Fatal("anonymous error schema emitted an empty component reference")
	}
	for _, typ := range []reflect.Type{reflect.TypeFor[Revision](), reflect.TypeFor[Release](), reflect.TypeFor[ReleaseForm](), reflect.TypeFor[ResolveRequest](), reflect.TypeFor[ResolvedExecution](), reflect.TypeFor[PreparedInput](), reflect.TypeFor[PreparedFile](), reflect.TypeFor[TargetDescriptor](), reflect.TypeFor[InputResource](), reflect.TypeFor[PublicationRequest]()} {
		properties := schemas[typ.Name()].(map[string]any)["properties"].(map[string]any)
		if schemas[typ.Name()].(map[string]any)["additionalProperties"] != false {
			t.Fatalf("open DTO %s", typ.Name())
		}
		for i := 0; i < typ.NumField(); i++ {
			name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			if _, ok := properties[name]; !ok {
				t.Errorf("missing %s.%s", typ.Name(), name)
			}
		}
	}
	expected, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile("../../openapi/projects-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(append(expected, '\n')) {
		t.Fatal("regenerate Projects OpenAPI")
	}
	selection := schemas["RenderSelectionRequest"].(map[string]any)
	if len(selection["oneOf"].([]any)) != 2 {
		t.Fatal("Projects resolver schema does not carry selector exclusivity")
	}
	generationSchemas := generationv1.OpenAPI()["components"].(map[string]any)["schemas"].(map[string]any)
	if !reflect.DeepEqual(selection, generationSchemas["RenderSelectionRequest"]) {
		t.Fatal("Projects and Generation selection schemas differ")
	}
}
