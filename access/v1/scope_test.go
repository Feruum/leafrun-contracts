package accessv1

import (
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	storagev1 "github.com/leafrun/leafrun-contracts/storage/v1"
	"strings"
	"testing"
	"time"
)

func TestPermissionScopeMatrix(t *testing.T) {
	for _, tc := range []struct {
		s                 PermissionScope
		canonical, legacy bool
	}{
		{PermissionScope{}, false, false},
		{PermissionScope{OrganizationID: "o"}, true, true},
		{PermissionScope{WorkspaceID: "w"}, false, false},
		{PermissionScope{ProjectID: "p"}, false, false},
		{PermissionScope{OrganizationID: "o", ProjectID: "p"}, false, false},
		{PermissionScope{OrganizationID: "o", WorkspaceID: "w"}, true, true},
		{PermissionScope{WorkspaceID: "w", ProjectID: "p"}, false, true},
		{PermissionScope{OrganizationID: "o", WorkspaceID: "w", ProjectID: "p"}, true, true},
		{PermissionScope{OrganizationID: " o", WorkspaceID: "w", ProjectID: "p"}, false, false},
		{PermissionScope{OrganizationID: "o", WorkspaceID: "w\n"}, false, false},
	} {
		if (tc.s.Validate() == nil) != tc.canonical {
			t.Errorf("canonical: %#v", tc)
		}
		if (tc.s.ValidateLegacy() == nil) != tc.legacy {
			t.Errorf("legacy: %#v", tc)
		}
	}
}

func TestResolveExpiryByAuthorityKind(t *testing.T) {
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	zero := time.Time{}
	for _, kind := range []string{identityv1.KindKratosSession, identityv1.KindGuestSession, identityv1.KindOAuthToken, identityv1.KindBotKey, identityv1.KindService, identityv1.KindDev} {
		bounded := kind == identityv1.KindKratosSession || kind == identityv1.KindGuestSession || kind == identityv1.KindOAuthToken
		for _, expiry := range []*time.Time{nil, &zero, &past, &now, &future} {
			out := ResolveResponse{Identity: identityv1.Identity{Principal: identityv1.Principal{Subject: "actor", Domain: "domain"}, Authority: identityv1.AuthorityRef{Kind: kind, ID: "authority-1", Version: 1}}, ExpiresAt: expiry}
			valid := expiry == &future || expiry == nil && !bounded
			if (out.Validate(now) == nil) != valid {
				t.Fatalf("kind %s expiry %v validity %v", kind, expiry, valid)
			}
		}
	}
}
func TestStorageConversionPreservesLegacyWire(t *testing.T) {
	in := storagev1.AccessCheck{Principal: storagev1.Principal{Subject: "alice", Domain: "user"}, Scope: storagev1.Scope{WorkspaceID: "w", ProjectID: "p"}, Operation: "resource.read", ObjectID: "r"}
	got := FromStorageAccessCheck(in)
	a, _ := json.Marshal(in)
	b, _ := json.Marshal(got)
	const want = `{"principal":{"subject":"alice","domain":"user"},"scope":{"workspaceId":"w","projectId":"p"},"operation":"resource.read","objectId":"r"}`
	if string(a) != want || string(b) != want {
		t.Fatalf("legacy wire changed: %s %s", a, b)
	}
	if err := got.Validate(); err != nil {
		t.Fatal(err)
	}
	authority := identityv1.AuthorityRef{Kind: identityv1.KindBotKey, ID: "k", Version: 1}
	in.Authority = &authority
	if FromStorageAccessCheck(in).Authority != in.Authority {
		t.Fatal("authority conversion changed reference")
	}
}
func TestResolveAndCheckValidationSanitizesCredentials(t *testing.T) {
	for _, in := range []ResolveRequest{{}, {Bearer: "secret", SessionCookie: "cookie"}, {Bearer: "secret\n"}, {Bearer: " secret"}, {Bearer: "a,b"}, {SessionCookie: "a;b"}, {Bearer: strings.Repeat("x", MaxCredentialBytes+1)}} {
		if err := in.Validate(); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("bad credential validation: %v", err)
		}
	}
	for _, in := range []ResolveRequest{{Bearer: "secret"}, {SessionCookie: "cookie"}} {
		if err := in.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	in := CheckRequest{Principal: identityv1.Principal{Subject: "a", Domain: "user"}, Scope: PermissionScope{OrganizationID: "o"}, Operation: "project.publish"}
	if err := in.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"", "a b", "a\n", strings.Repeat("a", 257)} {
		in.Operation = op
		if in.Validate() == nil {
			t.Errorf("accepted operation %q", op)
		}
	}
}
