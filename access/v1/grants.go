package accessv1

import (
	"encoding/base64"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"sort"
	"strings"
	"time"
)

type Grant struct {
	GrantID   string               `json:"grantId"`
	Scope     PermissionScope      `json:"scope"`
	Issuer    identityv1.Principal `json:"issuer"`
	Role      string               `json:"role,omitempty"`
	Version   uint64               `json:"version"`
	Status    string               `json:"status"`
	ExpiresAt time.Time            `json:"expiresAt"`
	Ceiling   CredentialCeiling    `json:"ceiling"`
}
type GrantResult struct {
	Grant      Grant  `json:"grant"`
	Secret     string `json:"secret,omitempty"`
	Disclosure string `json:"disclosure"`
}
type CreateGrantRequest struct {
	OperationKey string    `json:"operationKey"`
	Role         string    `json:"role,omitempty"`
	Operations   []string  `json:"operations,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
}
type RotateGrantRequest struct {
	OperationKey    string    `json:"operationKey"`
	Role            string    `json:"role,omitempty"`
	Operations      []string  `json:"operations,omitempty"`
	ExpiresAt       time.Time `json:"expiresAt"`
	ExpectedVersion uint64    `json:"expectedVersion"`
}
type RevokeGrantRequest struct {
	OperationKey    string `json:"operationKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type ExchangeGrantRequest struct {
	Secret  string `json:"secret"`
	CSRF    string `json:"csrf"`
	Replace bool   `json:"replace"`
}
type GuestLogoutRequest struct {
	CSRF string `json:"csrf"`
}

func GrantOperation(op string) bool {
	switch op {
	case "project.read", "comment.create", "project.write", "generation.cancel", "generation.create", "generation.execute", "generation.publish", "generation.read", "source.read", "resource.read", "artifact.read", "download":
		return true
	}
	return false
}
func GrantOperations(role string, operations []string) ([]string, error) {
	if len(operations) > 12 {
		return nil, ErrInvalid
	}
	seen := map[string]bool{}
	for _, op := range operations {
		if !GrantOperation(op) || seen[op] {
			return nil, ErrInvalid
		}
		seen[op] = true
	}
	switch role {
	case "":
	case "reader", "commenter", "editor":
		for _, op := range []string{"project.read", "generation.read", "source.read", "resource.read", "artifact.read", "download"} {
			seen[op] = true
		}
		if role != "reader" {
			seen["comment.create"] = true
		}
		if role == "editor" {
			for _, op := range []string{"project.write", "generation.cancel", "generation.create", "generation.execute", "generation.publish"} {
				seen[op] = true
			}
		}
	default:
		return nil, ErrInvalid
	}
	if len(seen) == 0 {
		return nil, ErrInvalid
	}
	out := make([]string, 0, len(seen))
	for op := range seen {
		out = append(out, op)
	}
	sort.Strings(out)
	return out, nil
}
func (r CreateGrantRequest) ValidateShape() error {
	if !identityv1.ValidIdentifier(r.OperationKey) || r.ExpiresAt.IsZero() {
		return ErrInvalid
	}
	_, err := GrantOperations(r.Role, r.Operations)
	return err
}
func (r CreateGrantRequest) Validate(now time.Time) error {
	if r.ValidateShape() != nil || !r.ExpiresAt.After(now) || r.ExpiresAt.After(now.Add(366*24*time.Hour)) {
		return ErrInvalid
	}
	return nil
}
func (r RotateGrantRequest) ValidateShape() error {
	if r.ExpectedVersion == 0 || r.ExpectedVersion >= identityv1.MaxAuthorityVersion {
		return ErrInvalid
	}
	return (CreateGrantRequest{OperationKey: r.OperationKey, Role: r.Role, Operations: r.Operations, ExpiresAt: r.ExpiresAt}).ValidateShape()
}
func (r RotateGrantRequest) Validate(now time.Time) error {
	if r.ValidateShape() != nil {
		return ErrInvalid
	}
	return (CreateGrantRequest{OperationKey: r.OperationKey, Role: r.Role, Operations: r.Operations, ExpiresAt: r.ExpiresAt}).Validate(now)
}
func (r RevokeGrantRequest) Validate() error {
	if !identityv1.ValidIdentifier(r.OperationKey) || r.ExpectedVersion == 0 || r.ExpectedVersion >= identityv1.MaxAuthorityVersion {
		return ErrInvalid
	}
	return nil
}
func (g Grant) Validate() error {
	if !identityv1.ValidIdentifier(g.GrantID) || g.Scope.Validate() != nil || g.Scope.ProjectID == "" || g.Issuer.Validate() != nil || g.Version == 0 || g.Version > identityv1.MaxAuthorityVersion || (g.Status != "active" && g.Status != "revoked") || g.ExpiresAt.IsZero() || g.Ceiling.Scope != g.Scope {
		return ErrInvalid
	}
	if _, err := GrantOperations("", g.Ceiling.Operations); err != nil {
		return err
	}
	if g.Role != "" && g.Role != "reader" && g.Role != "commenter" && g.Role != "editor" {
		return ErrInvalid
	}
	if g.Ceiling.Scope.ProjectID == "" {
		return ErrInvalid
	}
	if _, err := GrantOperations(g.Role, g.Ceiling.Operations); err != nil {
		return err
	}
	return nil
}
func ValidGrantSecret(secret string) bool {
	if !strings.HasPrefix(secret, "lr_grant_") || len(secret) != 52 {
		return false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(secret[9:])
	return err == nil && len(b) == 32
}
func (r GrantResult) Validate() error {
	if r.Grant.Validate() != nil {
		return ErrInvalid
	}
	if r.Disclosure == "unavailable" && r.Secret == "" {
		return nil
	}
	if r.Disclosure != "first" || r.Grant.Status != "active" || !ValidGrantSecret(r.Secret) {
		return ErrInvalid
	}
	return nil
}
func (r ExchangeGrantRequest) Validate() error {
	if !ValidGrantSecret(r.Secret) || !ValidCredential(r.CSRF) {
		return ErrInvalid
	}
	return nil
}
func (r GuestLogoutRequest) Validate() error {
	if !ValidCredential(r.CSRF) {
		return ErrInvalid
	}
	return nil
}
