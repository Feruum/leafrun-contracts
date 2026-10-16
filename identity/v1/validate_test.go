package identityv1

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPrincipalWireAndComparableIdentity(t *testing.T) {
	p := Principal{Subject: "alice", Domain: "user"}
	b, _ := json.Marshal(p)
	if string(b) != `{"subject":"alice","domain":"user"}` {
		t.Fatalf("principal bytes changed: %s", b)
	}
	_ = map[Identity]bool{{Principal: p, Authority: AuthorityRef{Kind: KindKratosSession, ID: "session-1", Version: 1}}: true}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestIdentityValidation(t *testing.T) {
	good := Identity{Principal: Principal{Subject: "alice", Domain: "user"}, Authority: AuthorityRef{Kind: KindKratosSession, ID: "session-1", Version: 1}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{KindKratosSession, KindBotKey, KindGuestSession, KindOAuthToken, KindService, KindDev} {
		a := good.Authority
		a.Kind = kind
		if err := a.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, bad := range []AuthorityRef{{Kind: "unknown", ID: "a", Version: 1}, {Kind: KindBotKey, ID: "", Version: 1}, {Kind: KindBotKey, ID: "a", Version: 0}, {Kind: KindBotKey, ID: "a", Version: 1 << 63}, {Kind: KindBotKey, ID: "a", Version: ^uint64(0)}} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("accepted %#v", bad)
		}
	}
	for _, id := range []string{"", " a", "a ", "a\n", "a\x00", "a\t", "\xff", strings.Repeat("x", 257), "a\u0085b"} {
		p := good.Principal
		p.Subject = id
		if p.Validate() == nil {
			t.Errorf("accepted subject %q", id)
		}
		a := good.Authority
		a.ID = id
		if a.Validate() == nil {
			t.Errorf("accepted authority ID %q", id)
		}
	}
	a := good.Authority
	a.ID = strings.Repeat("x", 256)
	a.Version = MaxAuthorityVersion
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
}
