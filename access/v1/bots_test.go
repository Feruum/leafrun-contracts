package accessv1

import (
	"testing"
	"time"
)

func TestBotSubjectKind(t *testing.T) {
	if (BindingRequest{SubjectKind: "bot", SubjectID: "owned-bot", Relation: "owner"}).Validate() != nil {
		t.Fatal("owned bot subject kind must be representable")
	}
}

func TestBotKeyInputBounds(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	good := BotKeyRequest{OperationKey: "issue", Operations: []string{"generation.create"}, ExpiresAt: &future}
	if good.Validate(now) != nil {
		t.Fatal("valid key input denied")
	}
	for _, ops := range [][]string{nil, {"generation.create", "generation.create"}, {"health"}, {"workspace.role.manage"}, {"organization.create"}, {"unknown"}} {
		bad := good
		bad.Operations = ops
		if bad.Validate(now) == nil {
			t.Fatal("unsupported or ambiguous ceiling accepted")
		}
	}
	expired := now
	good.ExpiresAt = &expired
	if good.Validate(now) == nil {
		t.Fatal("expired key allowed")
	}
	tooLong := now.Add(367 * 24 * time.Hour)
	good.ExpiresAt = &tooLong
	if good.Validate(now) == nil {
		t.Fatal("excess expiry accepted")
	}
}
