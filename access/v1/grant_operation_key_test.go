package accessv1

import (
	"errors"
	"testing"
	"time"
)

func TestGrantCreateKeyHasFiniteImmutableRetryWindow(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	key, e := NewGrantOperationKey(now)
	if e != nil {
		t.Fatal(e)
	}
	stamp, e := GrantOperationKeyTime(key)
	if e != nil || !stamp.Equal(now) {
		t.Fatal("key timestamp not immutable")
	}
	if ValidateGrantOperationKey(key, now) != nil {
		t.Fatal("fresh key rejected")
	}
	if !errors.Is(ValidateGrantOperationKey(key, now.Add(GrantOperationWindow)), ErrOperationExpired) {
		t.Fatal("expired key can issue again")
	}
	future, _ := NewGrantOperationKey(now.Add(2 * time.Minute))
	if ValidateGrantOperationKey(future, now) == nil {
		t.Fatal("future key accepted")
	}
	if _, e := GrantOperationKeyTime("legacy-key"); e == nil {
		t.Fatal("legacy key treated as new create")
	}
}
