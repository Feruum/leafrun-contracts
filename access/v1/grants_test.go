package accessv1

import (
	"encoding/base64"
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
	"reflect"
	"testing"
	"time"
)

func TestGrantClosedDelegationAndFiniteExpiry(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	for _, op := range []string{"grant.manage", "role.manage", "project.publish", "bot.manage", "audit.read", "organization.create", "health", "generate"} {
		if _, err := GrantOperations("", []string{op}); err == nil {
			t.Errorf("delegated %s", op)
		}
	}
	for _, role := range []string{"owner", "admin", "Reader"} {
		if _, err := GrantOperations(role, nil); err == nil {
			t.Errorf("delegated role %s", role)
		}
	}
	if _, err := GrantOperations("", []string{"download", "download"}); err == nil {
		t.Fatal("duplicate operation accepted")
	}
	got, err := GrantOperations("reader", []string{"generation.create"})
	if err != nil || !reflect.DeepEqual(got, []string{"artifact.read", "download", "generation.create", "generation.read", "project.read", "resource.read", "source.read"}) {
		t.Fatalf("reader union: %v %v", got, err)
	}
	for _, expiry := range []time.Time{{}, now, now.Add(-time.Second), now.Add(367 * 24 * time.Hour)} {
		if (CreateGrantRequest{OperationKey: "op", Role: "reader", ExpiresAt: expiry}).Validate(now) == nil {
			t.Fatal("unbounded expiry accepted")
		}
	}
	if (CreateGrantRequest{OperationKey: "op", Role: "reader", ExpiresAt: now.Add(time.Hour)}).Validate(now) != nil {
		t.Fatal("finite grant denied")
	}
	if (CreateGrantRequest{OperationKey: "op", Role: "reader", ExpiresAt: now.Add(-time.Hour)}).ValidateShape() != nil {
		t.Fatal("expired receipt shape denied")
	}
}
func TestGrantDisclosureAndStrictWire(t *testing.T) {
	scope := PermissionScope{OrganizationID: "org", WorkspaceID: "ws", ProjectID: "project"}
	ops, _ := GrantOperations("reader", nil)
	grant := Grant{GrantID: "grant", Scope: scope, Issuer: identityv1.Principal{Subject: "alice", Domain: "human"}, Role: "reader", Version: 1, Status: "active", ExpiresAt: time.Now().Add(time.Hour), Ceiling: CredentialCeiling{Scope: scope, Operations: ops}}
	secret := "lr_grant_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, result := range []GrantResult{{Grant: grant, Disclosure: "first", Secret: secret}, {Grant: grant, Disclosure: "unavailable"}} {
		if result.Validate() != nil {
			t.Fatal("valid disclosure denied")
		}
	}
	for _, result := range []GrantResult{{Grant: grant, Disclosure: "first"}, {Grant: grant, Disclosure: "unavailable", Secret: secret}, {Grant: grant, Disclosure: "first", Secret: "lr_guest_" + secret[9:]}} {
		if result.Validate() == nil {
			t.Fatal("secret disclosure violation")
		}
	}
	for _, raw := range []string{`{"operationKey":"op","role":"reader","role":"editor","expiresAt":"2099-01-01T00:00:00Z"}`, `{"operationKey":"op","Role":"reader","expiresAt":"2099-01-01T00:00:00Z"}`, `{"operationKey":"op","role":"reader","actor":"forged","expiresAt":"2099-01-01T00:00:00Z"}`} {
		var in CreateGrantRequest
		if jsonvalue.Unmarshal([]byte(raw), &in, 4096) == nil {
			t.Fatal("open grant wire accepted")
		}
	}
	for _, kind := range []SessionCookieKind{"", CookieKindKratos, CookieKindGuest} {
		in := ResolveRequest{SessionCookie: "opaque", SessionCookieKind: kind}
		if in.Validate() != nil {
			t.Fatal("valid cookie kind rejected")
		}
		b, _ := json.Marshal(in)
		var out ResolveRequest
		if jsonvalue.Unmarshal(b, &out, 4096) != nil || out != in {
			t.Fatal("kind wire mismatch")
		}
	}
}

func TestGrantMetadataRequiresExplicitNonemptyCeiling(t *testing.T) {
	scope := PermissionScope{OrganizationID: "org", WorkspaceID: "ws", ProjectID: "project"}
	g := Grant{GrantID: "grant", Scope: scope, Issuer: identityv1.Principal{Subject: "alice", Domain: "human"}, Role: "reader", Version: 1, Status: "active", ExpiresAt: time.Now().Add(time.Hour), Ceiling: CredentialCeiling{Scope: scope}}
	if g.Validate() == nil {
		t.Fatal("empty explicit issued ceiling accepted")
	}
}

func TestGrantOpenAPIBoundsExplicitCeilingAndVersion(t *testing.T) {
	schemas := NewOpenAPI()["components"].(map[string]any)["schemas"].(map[string]any)
	props := schemas["Grant"].(map[string]any)["properties"].(map[string]any)
	ceiling := props["ceiling"].(map[string]any)
	if _, ok := ceiling["allOf"]; !ok {
		t.Fatal("grant ceiling schema lacks closed guest operation bound")
	}
	for _, name := range []string{"RotateGrantRequest", "RevokeGrantRequest"} {
		version := schemas[name].(map[string]any)["properties"].(map[string]any)["expectedVersion"].(map[string]any)
		if version["maximum"] != identityv1.MaxAuthorityVersion-1 {
			t.Fatal("grant mutation version overflow allowed by schema")
		}
	}
}
