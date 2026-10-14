package jsonvalue

import (
	"encoding/json"
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
