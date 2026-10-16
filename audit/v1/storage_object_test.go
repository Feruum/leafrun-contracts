package auditv1

import (
	"testing"
)

func TestDownloadAuthorizationHasTruthfulStorageObjectKind(t *testing.T) {
	a := actionFixture()
	a.Resource.Kind = ResourceStorageObject
	subject, _ := Subject(a.Producer)
	raw, e := EncodeAction(a)
	if e != nil {
		t.Fatal(e)
	}
	out, e := DecodeAction(subject, raw)
	if e != nil || out.Resource.Kind != ResourceStorageObject {
		t.Fatal("storage object kind not retained")
	}
}
