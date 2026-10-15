package jsonvalue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStrictUnmarshalRejectsAliasesAndDuplicateUnicodeKeys(t *testing.T) {
	type scope struct {
		WorkspaceID string `json:"workspaceId"`
	}
	type message struct {
		Scope   scope           `json:"scope"`
		Payload json.RawMessage `json:"payload"`
	}
	for _, input := range []string{`{"Scope":{"workspaceId":"w"}}`, `{"scope":{"WorkspaceID":"w"}}`, `{"scope":{"workspaceId":"a","\u0077orkspaceId":"b"}}`, `{"scope":{"workspaceId":"\ud800"}}`} {
		var out message
		if err := Unmarshal([]byte(input), &out, 1<<20); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	var out message
	if err := Unmarshal([]byte(`{"scope":{"workspaceId":"w"},"payload":{"arbitrary":1.0}}`), &out, 1<<20); err != nil {
		t.Fatal(err)
	}
	if string(out.Payload) != `{"arbitrary":1.0}` {
		t.Fatal("changed raw message")
	}
}
func TestStrictDecodePreservesSchemaNumbersWithoutParameterBounds(t *testing.T) {
	value, err := Decode([]byte(`{"minimum":9223372036854775808}`), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.(map[string]any)["minimum"]; got != json.Number("9223372036854775808") {
		t.Fatalf("schema number coerced: %#v", got)
	}
}

func TestStrictDecodeBoundsWireBytesWithoutChangingCanonicalEncoding(t *testing.T) {
	input := []byte(`{"description":"<>&` + "\u2028\u2029" + `","default":1.2300e+04}`)
	limit := int64(len(input))
	value, err := Decode(input, limit)
	if err != nil {
		t.Fatalf("valid JSON within wire byte cap rejected: %v", err)
	}
	if value.(map[string]any)["default"] != json.Number("1.2300e+04") {
		t.Fatal("numeric lexeme changed")
	}
	var out struct {
		Description string      `json:"description"`
		Default     json.Number `json:"default"`
	}
	if err = Unmarshal(input, &out, limit); err != nil {
		t.Fatalf("strict wire DTO rejected: %v", err)
	}
	if _, err = Decode(input, limit-1); err == nil {
		t.Fatal("accepted bytes beyond wire cap")
	}
	if _, err = NormalizeLimit(input, limit); err == nil {
		t.Fatal("canonical normalization no longer enforces output byte cap")
	}
	canonical, err := NormalizeLimit(input, 1024)
	want := `{"default":1.2300e+04,"description":"\u003c\u003e\u0026\u2028\u2029"}`
	if err != nil || string(canonical) != want {
		t.Fatalf("canonical identity changed: %s, %v", canonical, err)
	}
}

func TestStrictDecodeRetainsWireValidation(t *testing.T) {
	for _, input := range []string{
		`{"a":1,"\u0061":2}`, `{"x":"\ud800"}`, "{\"x\":\"\xff\"}",
		`{"x":true} false`, `{"x":true,}`, `{"n":01}`,
		strings.Repeat("[", MaxDepth+1) + "0" + strings.Repeat("]", MaxDepth+1),
	} {
		if _, err := Decode([]byte(input), 1<<20); err == nil {
			t.Errorf("accepted invalid JSON %q", input)
		}
	}
}
