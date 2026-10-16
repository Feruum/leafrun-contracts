package accessv1

import (
	"bytes"
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestOpenAPITracksDTOsAndBytes(t *testing.T) {
	spec := NewOpenAPI()
	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	for _, typ := range []reflect.Type{reflect.TypeFor[ResolveRequest](), reflect.TypeFor[ResolveResponse](), reflect.TypeFor[CheckRequest](), reflect.TypeFor[CheckResponse](), reflect.TypeFor[PermissionScope](), reflect.TypeFor[identityv1.Principal](), reflect.TypeFor[identityv1.AuthorityRef](), reflect.TypeFor[identityv1.Identity]()} {
		s := schemas[typ.Name()].(map[string]any)
		if s["additionalProperties"] != false {
			t.Fatalf("open object %s", typ.Name())
		}
		p := s["properties"].(map[string]any)
		for i := 0; i < typ.NumField(); i++ {
			name := strings.Split(typ.Field(i).Tag.Get("json"), ",")[0]
			if _, ok := p[name]; !ok {
				t.Fatalf("missing %s.%s", typ.Name(), name)
			}
		}
	}
	if len(schemas["ResolveRequest"].(map[string]any)["oneOf"].([]any)) != 2 {
		t.Fatal("credential exclusivity missing")
	}
	if len(schemas["PermissionScope"].(map[string]any)["oneOf"].([]any)) != 4 {
		t.Fatal("scope matrix missing")
	}
	authority := schemas["AuthorityRef"].(map[string]any)["properties"].(map[string]any)
	if authority["version"].(map[string]any)["maximum"] != identityv1.MaxAuthorityVersion {
		t.Fatal("version bound missing")
	}
	if len(authority["kind"].(map[string]any)["enum"].([]string)) != 6 {
		t.Fatal("kind enum missing")
	}
	if schemas["CheckRequest"].(map[string]any)["properties"].(map[string]any)["operation"].(map[string]any)["pattern"] != operationPattern.String() {
		t.Fatal("operation syntax missing")
	}
	want, _ := json.MarshalIndent(spec, "", "  ")
	got, err := os.ReadFile("../../openapi/access-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, append(want, '\n')) {
		t.Fatal("regenerate Access OpenAPI")
	}
}

func TestOpenAPIPublicSessionIsSeparateFromServiceCredential(t *testing.T) {
	spec := NewOpenAPI()
	route, ok := spec["paths"].(map[string]any)["/v1/session"].(map[string]any)
	if !ok {
		t.Fatal("public session route absent")
	}
	get := route["get"].(map[string]any)
	security := get["security"].([]any)
	if len(security) != 3 {
		t.Fatal("public credential alternatives absent")
	}
	for _, s := range security {
		if _, ok := s.(map[string]any)["serviceBearer"]; ok {
			t.Fatal("service bearer on public route")
		}
	}
}

func TestOpenAPIIdentifierSyntaxPreservesUnicode(t *testing.T) {
	schemas := NewOpenAPI()["components"].(map[string]any)["schemas"].(map[string]any)
	for schema, fields := range map[string][]string{"Principal": {"subject", "domain"}, "AuthorityRef": {"id"}, "PermissionScope": {"organizationId", "workspaceId", "projectId"}, "CheckRequest": {"objectId"}} {
		properties := schemas[schema].(map[string]any)["properties"].(map[string]any)
		for _, field := range fields {
			t.Run(schema+"."+field, func(t *testing.T) {
				property := properties[field].(map[string]any)
				pattern, ok := property["pattern"].(string)
				if !ok {
					t.Fatal("identifier trim/control syntax is absent")
				}
				re, err := regexp.Compile(pattern)
				if err != nil {
					t.Fatal(err)
				}
				// ECMA-262 $ can match before a final line terminator. The explicit
				// forbidden-character/boundary rule closes that schema-engine edge.
				not, ok := property["not"].(map[string]any)
				if !ok {
					t.Fatal("strict Unicode boundary rule is absent")
				}
				forbidden, err := regexp.Compile(not["pattern"].(string))
				if err != nil {
					t.Fatal(err)
				}
				accepts := func(value string) bool { return re.MatchString(value) && !forbidden.MatchString(value) }
				for _, value := range []string{" alice", "alice ", "\u00a0alice", "alice\u3000", "alice\u2028", "alice\u2029", "key\nadmin", "a\x00b", "a\u0085b", "\tactor", "", "name\r"} {
					if accepts(value) {
						t.Errorf("schema accepts padded/control ID %q", value)
					}
				}
				for _, value := range []string{"名字 项目", "Жоба １２３", "café", "a\u00a0b", "a\u2028b", "😀", "a\ufeffb", "a b"} {
					if !accepts(value) || !identityv1.ValidIdentifier(value) {
						t.Errorf("schema or Go rejects valid Unicode/interior-space ID %q", value)
					}
				}
				description, _ := property["description"].(string)
				for _, required := range []string{"UTF-8 bytes", "Unicode whitespace", "control", "characters"} {
					if !strings.Contains(description, required) {
						t.Errorf("ID description omits %q", required)
					}
				}
				if property["x-max-utf8-bytes"] != 256 || property["maxLength"] != 256 {
					t.Fatal("byte/character bounds changed")
				}
			})
		}
	}
}
