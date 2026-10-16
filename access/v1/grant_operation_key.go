package accessv1

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrOperationExpired = errors.New("operation retry window expired")

const GrantOperationClockSkew = time.Minute

// GrantOperationWindow is immutable for the UUIDv7 v1 create-key protocol.
// Increasing configurable metadata retention never extends key validity.
const GrantOperationWindow = 30 * 24 * time.Hour

// NewGrantOperationKey creates an immutable UUIDv7 retry key for a NEW grant
// creation. Persist it before sending. Retries reuse the same key and body;
// after the fixed v1 30-day retry window the server rejects it even after cleanup.
// It is private deduplication input and must never become an audit operation ID.
func NewGrantOperationKey(now time.Time) (string, error) {
	ms := now.UnixMilli()
	if ms <= 0 || ms >= 1<<48 {
		return "", ErrInvalid
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return "", ErrDependency
	}
	for i := 5; i >= 0; i-- {
		b[i] = byte(ms)
		ms >>= 8
	}
	b[6] = b[6]&15 | 0x70
	b[8] = b[8]&63 | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
func GrantOperationKeyTime(key string) (time.Time, error) {
	if len(key) != 36 || key != strings.ToLower(key) || key[8] != '-' || key[13] != '-' || key[18] != '-' || key[23] != '-' {
		return time.Time{}, ErrInvalid
	}
	b, e := hex.DecodeString(strings.ReplaceAll(key, "-", ""))
	if e != nil || len(b) != 16 || b[6]>>4 != 7 || b[8]>>6 != 2 {
		return time.Time{}, ErrInvalid
	}
	var ms int64
	for i := 0; i < 6; i++ {
		ms = ms<<8 | int64(b[i])
	}
	if ms <= 0 {
		return time.Time{}, ErrInvalid
	}
	return time.UnixMilli(ms).UTC(), nil
}

// ValidateGrantOperationKey checks a proposed new-create key against a clock.
// The owner must additionally apply this rule with the final database clock
// after lock waits and preserve current authorization immediately before commit.
// Legacy keys are replay-only under an existing unexpired matching receipt.
func ValidateGrantOperationKey(key string, now time.Time) error {
	born, e := GrantOperationKeyTime(key)
	if e != nil || now.IsZero() || born.After(now.Add(GrantOperationClockSkew)) {
		return ErrInvalid
	}
	if !born.Add(GrantOperationWindow).After(now) {
		return ErrOperationExpired
	}
	return nil
}
