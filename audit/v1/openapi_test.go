package auditv1

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestAuditOpenAPIByteMatch(t *testing.T) {
	data, e := json.MarshalIndent(NewOpenAPI(), "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	data = append(data, '\n')
	saved, e := os.ReadFile("../../openapi/audit-v1.json")
	if e != nil || !bytes.Equal(saved, data) {
		t.Fatal("regenerate Audit OpenAPI")
	}
}
