package generationv1

import (
	"strings"
	"testing"
)

func TestNormalizeRequestFingerprint(t *testing.T) {
	a := []byte(`{"scope":{"workspaceId":"w","projectId":"p"},"snapshotId":"s","targetId":"t","entrypoint":"main.typ","parameters":{"z":1.0,"a":9007199254740993},"parametersSchemaPath":"schemas/input.json"}`)
	b := []byte(` { "entrypoint":"main.typ", "targetId":"t", "parametersSchemaPath":"schemas/input.json", "snapshotId":"s","parameters":{"a":9007199254740993,"z":1.0}, "scope":{"projectId":"p","workspaceId":"w"} }`)
	r, normalized, fingerprint, err := NormalizeRequest(a)
	if err != nil {
		t.Fatal(err)
	}
	_, other, fp, err := NormalizeRequest(b)
	if err != nil || string(normalized) != string(other) || fp != fingerprint {
		t.Fatal("equivalent payload differs", err)
	}
	if string(r.Parameters) != `{"a":9007199254740993,"z":1.0}` {
		t.Fatal("parameters changed", string(r.Parameters))
	}
	for _, changed := range []string{strings.Replace(string(a), `1.0`, `1`, 1), strings.Replace(string(a), `"p"`, `"q"`, 1), strings.Replace(string(a), `"s"`, `"s2"`, 1)} {
		_, _, fp, err := NormalizeRequest([]byte(changed))
		if err != nil || fp == fingerprint {
			t.Fatal("changed request shares fingerprint", err)
		}
	}
}
func TestNormalizeRequestRejectsStructure(t *testing.T) {
	base := `{"scope":{"workspaceId":"w","projectId":"p"},"snapshotId":"s","targetId":"t","entrypoint":"main.typ"}`
	for _, in := range []string{
		strings.Replace(base, `"main.typ"`, `"../main.typ"`, 1), strings.Replace(base, `"main.typ"`, `"C:\\main.typ"`, 1),
		strings.Replace(base, `"targetId":"t",`, "", 1), strings.Replace(base, `"projectId":"p"`, `"projectId":""`, 1),
		strings.TrimSuffix(base, "}") + `,"extra":true}`, strings.TrimSuffix(base, "}") + `,"parameters":{}}`,
		strings.TrimSuffix(base, "}") + `,"parametersSchemaPath":"schema.json"}`, strings.TrimSuffix(base, "}") + `,"parameters":null,"parametersSchemaPath":"schema.json"}`,
		strings.TrimSuffix(base, "}") + `,"parameters":[],"parametersSchemaPath":"schema.json"}`,
		strings.TrimSuffix(base, "}") + `,"parameters":{},"parametersSchemaPath":"https://evil.test/schema"}`,
		strings.TrimSuffix(base, "}") + `,"parameters":{},"parametersSchemaPath":"input.json/../schema.json"}`,
		strings.TrimSuffix(base, "}") + `,"parametersSchemaPath":null}`,
		strings.TrimSuffix(base, "}") + `,"parametersSchemaPath":""}`,
		strings.Replace(base, `"scope"`, `"Scope"`, 1),
		strings.Replace(base, `"workspaceId"`, `"WorkspaceID"`, 1),
		strings.TrimSuffix(base, "}") + `,"SnapshotId":"other"}`,
	} {
		if _, _, _, err := NormalizeRequest([]byte(in)); err == nil {
			t.Errorf("accepted %s", in)
		}
	}
	if _, _, _, err := NormalizeRequest([]byte(base)); err != nil {
		t.Fatal(err)
	}
	colonPath := strings.Replace(base, `"main.typ"`, `"src:archive/main.typ"`, 1)
	if _, _, _, err := NormalizeRequest([]byte(colonPath)); err != nil {
		t.Fatal("valid Linux logical path", err)
	}
}
