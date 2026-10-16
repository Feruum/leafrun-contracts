package accessv1

import (
	"context"
	"encoding/base64"
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestTypedGuestSessionUsesOnlyExplicitSelectedName(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Cookie") != "custom_guest=opaque" || r.Header.Get("Authorization") != "" {
			t.Error("wrong selected cookie")
		}
		if r.Method == "POST" && r.Header.Get("Origin") != "https://app.example" {
			t.Error("unsafe cookie origin missing")
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(ResolveResponse{Identity: identityv1.Identity{Principal: identityv1.Principal{Subject: "grant", Domain: "guest"}, Authority: identityv1.AuthorityRef{Kind: identityv1.KindGuestSession, ID: "session", Version: 1}}, ExpiresAt: func() *time.Time { x := time.Now().Add(time.Hour); return &x }()})
		} else {
			w.Write([]byte(`{}`))
		}
	}))
	defer s.Close()
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse(s.URL)
	jar.SetCookies(u, []*http.Cookie{{Name: "custom_human", Value: "ambient"}})
	c, err := NewSessionClientWithCookieNames(s.URL, "custom_human", "custom_guest", "https://app.example", false, &http.Client{Jar: jar})
	if err != nil {
		t.Fatal(err)
	}
	auth := ResolveRequest{SessionCookie: "opaque", SessionCookieKind: CookieKindGuest}
	if _, err := c.Session(context.Background(), auth); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := c.publicCall(context.Background(), auth, "POST", "/v1/action", map[string]any{}, &out); err != nil {
		t.Fatal(err)
	}
	legacy, _ := NewSessionClient(s.URL, "custom_human", nil)
	if _, err := legacy.Session(context.Background(), auth); err == nil {
		t.Fatal("guest silently mapped to human cookie")
	}
	if calls != 2 {
		t.Fatal("unsupported guest selection sent")
	}
	if _, err := NewSessionClientWithCookieNames(s.URL, "same", "same", "https://app.example", false, nil); err == nil {
		t.Fatal("colliding cookie names accepted")
	}
}
func TestTypedGrantManagementAndMetadataOnlyRetry(t *testing.T) {
	scope := PermissionScope{OrganizationID: "org", WorkspaceID: "ws", ProjectID: "project"}
	ops, _ := GrantOperations("reader", nil)
	expiry := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	grant := Grant{GrantID: "grant", Scope: scope, Issuer: identityv1.Principal{Subject: "alice", Domain: "human"}, Role: "reader", Version: 1, Status: "active", ExpiresAt: expiry, Ceiling: CredentialCeiling{Scope: scope, Operations: ops}}
	secret := "lr_grant_" + base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer human" || r.Header.Get("Cookie") != "" {
			t.Error("unexpected public management authentication")
		}
		result := GrantResult{Grant: grant, Disclosure: "unavailable"}
		switch calls {
		case 1:
			if r.URL.Path != "/v1/workspaces/ws/projects/project/grants" || r.Method != "POST" {
				t.Error("wrong create path")
			}
			result.Disclosure = "first"
			result.Secret = secret
		case 2:
			if r.URL.Path != "/v1/workspaces/ws/projects/project/grants" {
				t.Error("wrong retry path")
			}
		case 3:
			result.Grant.Version = 2
			if r.URL.Path != "/v1/grants/grant/rotate" {
				t.Error("wrong rotate path")
			}
			result.Disclosure = "first"
			result.Secret = secret
		case 4:
			result.Grant.Version = 3
			result.Grant.Status = "revoked"
			if r.Method != "DELETE" || r.URL.Path != "/v1/grants/grant" {
				t.Error("wrong revoke path")
			}
		case 5:
			result.Grant.Scope.ProjectID = "foreign"
			result.Grant.Ceiling.Scope = result.Grant.Scope
		}
		json.NewEncoder(w).Encode(result)
	}))
	defer s.Close()
	c, _ := NewSessionClient(s.URL, "human_cookie", nil)
	auth := ResolveRequest{Bearer: "human"}
	in := CreateGrantRequest{OperationKey: "create", Role: "reader", ExpiresAt: expiry}
	if out, err := c.CreateGrant(context.Background(), auth, "ws", "project", in); err != nil || out.Secret != secret {
		t.Fatalf("first disclosure: %v", err)
	}
	if out, err := c.CreateGrant(context.Background(), auth, "ws", "project", in); err != nil || out.Secret != "" || out.Disclosure != "unavailable" {
		t.Fatalf("metadata retry: %v", err)
	}
	if _, err := c.RotateGrant(context.Background(), auth, "grant", RotateGrantRequest{OperationKey: "rotate", ExpectedVersion: 1, Role: "reader", ExpiresAt: expiry}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RevokeGrant(context.Background(), auth, "grant", RevokeGrantRequest{OperationKey: "revoke", ExpectedVersion: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateGrant(context.Background(), auth, "ws", "project", in); err == nil {
		t.Fatal("foreign result scope accepted")
	}
	if _, err := c.CreateGrant(context.Background(), auth, "ws", "project", CreateGrantRequest{}); err == nil {
		t.Fatal("invalid grant request sent")
	}
	if calls != 5 {
		t.Fatal("invalid shape reached dependency")
	}
}
