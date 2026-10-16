package accessv1

import (
	"encoding/json"
	"testing"
)

func TestSelectedCookieKindRejectsBearerMetadata(t *testing.T) {
	var in ResolveRequest
	if err := json.Unmarshal([]byte(`{"bearer":"opaque","sessionCookieKind":"guest"}`), &in); err != nil {
		t.Fatal(err)
	}
	if in.Validate() == nil {
		t.Fatal("bearer with cookie kind must be rejected")
	}
}

func TestSelectedCookieKindRejectsExplicitEmptyAndNull(t *testing.T) {
	for _, raw := range []string{`{"sessionCookie":"opaque","sessionCookieKind":""}`, `{"sessionCookie":"opaque","sessionCookieKind":null}`, `{"sessionCookie":"opaque","sessionCookieKind":"provider"}`} {
		var in ResolveRequest
		if json.Unmarshal([]byte(raw), &in) == nil && in.Validate() == nil {
			t.Errorf("invalid explicit kind accepted: %s", raw)
		}
	}
}
