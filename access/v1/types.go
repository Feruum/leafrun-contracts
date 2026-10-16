// Package accessv1 defines Access transport contracts without permission policy.
package accessv1

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
)

const MaxCredentialBytes = 4096

// SessionCookieKind is bounded transport metadata derived from a configured cookie name.
// It selects no arbitrary provider, actor or authority.
type SessionCookieKind string

const (
	CookieKindKratos SessionCookieKind = "kratos"
	CookieKindGuest  SessionCookieKind = "guest"
)

// UnmarshalJSON distinguishes legacy omission from an explicitly invalid empty/null kind.
func (k *SessionCookieKind) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value != "kratos" && value != "guest" {
		return errors.New("invalid selected cookie kind")
	}
	*k = SessionCookieKind(value)
	return nil
}

// ResolveRequest carries exactly one public credential value, never a Cookie header.
// Omitted cookie kind preserves legacy Kratos. Internal service bearer stays separate.
type ResolveRequest struct {
	SessionCookieKind SessionCookieKind `json:"sessionCookieKind,omitempty"`
	Bearer            string            `json:"bearer,omitempty"`
	SessionCookie     string            `json:"sessionCookie,omitempty"`
}
type ResolveResponse struct {
	Identity identityv1.Identity `json:"identity"`
	// Nil explicitly permits a nonexpiring bot/service/dev authority. Session,
	// guest and OAuth identities always require a finite future expiry.
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}
type CheckRequest struct {
	Principal identityv1.Principal     `json:"principal"`
	Authority *identityv1.AuthorityRef `json:"authority,omitempty"`
	Scope     PermissionScope          `json:"scope"`
	Operation string                   `json:"operation"`
	ObjectID  string                   `json:"objectId,omitempty"`
}
type CheckResponse struct {
	Allowed bool `json:"allowed"`
	// CanonicalScope is supplied only for an allowed, registry-bound check.
	CanonicalScope *PermissionScope `json:"canonicalScope,omitempty"`
}

// ValidCredential bounds opaque values and rejects ambiguous header/cookie syntax.
func ValidCredential(value string) bool {
	if value == "" || len(value) > MaxCredentialBytes || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.IsSpace(r) || strings.ContainsRune(",;", r) {
			return false
		}
	}
	return true
}
func (r ResolveRequest) Validate() error {
	if r.SessionCookieKind != "" && (r.SessionCookie == "" || (r.SessionCookieKind != CookieKindKratos && r.SessionCookieKind != CookieKindGuest)) {
		return errors.New("invalid selected cookie kind")
	}
	if (r.Bearer == "") == (r.SessionCookie == "") {
		return errors.New("exactly one caller credential is required")
	}
	if r.Bearer != "" && !ValidCredential(r.Bearer) || r.SessionCookie != "" && !ValidCredential(r.SessionCookie) {
		return errors.New("invalid caller credential")
	}
	return nil
}
func (r ResolveResponse) Validate(now time.Time) error {
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	if r.ExpiresAt == nil {
		switch r.Identity.Authority.Kind {
		case identityv1.KindBotKey, identityv1.KindService, identityv1.KindDev:
			return nil
		}
		return errors.New("identity expiry is required")
	}
	if r.ExpiresAt.IsZero() || !r.ExpiresAt.After(now) {
		return errors.New("identity expiry must be in the future")
	}
	return nil
}

var operationPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)*$`)

func (r CheckRequest) Validate() error {
	if r.Operation == "generation.cleanup" && (r.Principal.Domain != "service" || r.Authority != nil || r.Scope.WorkspaceID == "" || r.Scope.ProjectID == "") {
		return errors.New("invalid service cleanup check")
	}
	if err := r.Principal.Validate(); err != nil {
		return err
	}
	if r.Authority != nil {
		if err := r.Authority.Validate(); err != nil {
			return err
		}
	}
	if err := r.Scope.ValidateLegacy(); err != nil && !(r.Principal.Domain == "service" && r.Operation == "health" && r.Scope == (PermissionScope{})) {
		return err
	}
	if len(r.Operation) > 256 || !operationPattern.MatchString(r.Operation) {
		return errors.New("invalid operation identifier")
	}
	if _, ok := Permit(r.Operation); !ok {
		return errors.New("unknown permission operation")
	}
	if r.ObjectID != "" && !identityv1.ValidIdentifier(r.ObjectID) {
		return errors.New("invalid object ID")
	}
	return nil
}
