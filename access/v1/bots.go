package accessv1

import (
	"encoding/base64"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"strings"
	"time"
)

// Bots have immutable exact home projects. Creating a bot grants no role or key;
// owners assign its role through the existing exact-project binding API.
type Bot struct {
	BotID string          `json:"botId"`
	Scope PermissionScope `json:"scope"`
	Name  string          `json:"name"`
}
type CreateBotRequest struct {
	OperationKey string `json:"operationKey"`
	Name         string `json:"name"`
}
type BotKeyRequest struct {
	OperationKey string     `json:"operationKey"`
	Operations   []string   `json:"operations"`
	ExpiresAt    *time.Time `json:"expiresAt,omitempty"`
}
type RotateBotKeyRequest struct {
	OperationKey    string     `json:"operationKey"`
	Operations      []string   `json:"operations"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	ExpectedVersion uint64     `json:"expectedVersion"`
}
type RevokeBotKeyRequest struct {
	OperationKey    string `json:"operationKey"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}
type BotKey struct {
	BotID     string            `json:"botId"`
	KeyID     string            `json:"keyId"`
	Version   uint64            `json:"version"`
	Status    string            `json:"status"`
	ExpiresAt *time.Time        `json:"expiresAt,omitempty"`
	Ceiling   CredentialCeiling `json:"ceiling"`
}

// Secret is present only in the successful first disclosure. A committed retry
// has disclosure=unavailable and the identical public metadata. Recover a lost
// disclosure by rotating with a fresh operationKey, never by retrying issuance.
type BotKeyResult struct {
	Key        BotKey `json:"key"`
	Disclosure string `json:"disclosure"`
	Secret     string `json:"secret,omitempty"`
}

func (r CreateBotRequest) Validate() error {
	if !identityv1.ValidIdentifier(r.OperationKey) || (NameRequest{Name: r.Name}).Validate() != nil {
		return ErrInvalid
	}
	return nil
}
func BotOperation(op string) bool {
	switch op {
	case "project.read", "comment.create", "project.write", "generation.cancel", "project.publish", "role.manage", "bot.manage", "grant.manage", "audit.read", "generation.create", "generation.execute", "generation.publish", "generation.read", "source.read", "resource.read", "artifact.read", "download":
		return true
	}
	return false
}
func (r BotKeyRequest) Validate(now time.Time) error {
	if r.ValidateShape() != nil {
		return ErrInvalid
	}
	if r.ExpiresAt != nil && (!r.ExpiresAt.After(now) || r.ExpiresAt.After(now.Add(366*24*time.Hour))) {
		return ErrInvalid
	}
	return nil
}

// Shape validation also admits the unchanged inputs of a committed receipt
// whose issued key has since expired. The owner validates future expiry only
// when applying a fresh mutation, after checking its durable receipt.
func (r BotKeyRequest) ValidateShape() error {
	if !identityv1.ValidIdentifier(r.OperationKey) || len(r.Operations) == 0 || len(r.Operations) > 17 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, op := range r.Operations {
		if !BotOperation(op) || seen[op] {
			return ErrInvalid
		}
		seen[op] = true
	}
	if r.ExpiresAt != nil && r.ExpiresAt.IsZero() {
		return ErrInvalid
	}
	return nil
}
func (r RotateBotKeyRequest) Validate(now time.Time) error {
	if r.ExpectedVersion == 0 || r.ExpectedVersion > 9223372036854775806 {
		return ErrInvalid
	}
	return (BotKeyRequest{OperationKey: r.OperationKey, Operations: r.Operations, ExpiresAt: r.ExpiresAt}).Validate(now)
}
func (r RotateBotKeyRequest) ValidateShape() error {
	if r.ExpectedVersion == 0 || r.ExpectedVersion > 9223372036854775806 {
		return ErrInvalid
	}
	return (BotKeyRequest{OperationKey: r.OperationKey, Operations: r.Operations, ExpiresAt: r.ExpiresAt}).ValidateShape()
}
func (r RevokeBotKeyRequest) Validate() error {
	if !identityv1.ValidIdentifier(r.OperationKey) || r.ExpectedVersion == 0 || r.ExpectedVersion > 9223372036854775806 {
		return ErrInvalid
	}
	return nil
}

func (k BotKey) Validate() error {
	if !identityv1.ValidIdentifier(k.BotID) || !identityv1.ValidIdentifier(k.KeyID) || k.Version == 0 || k.Version > identityv1.MaxAuthorityVersion || (k.Status != "active" && k.Status != "revoked") || k.Ceiling.Scope.Validate() != nil || k.Ceiling.Scope.ProjectID == "" {
		return ErrInvalid
	}
	return (BotKeyRequest{OperationKey: "metadata", Operations: k.Ceiling.Operations, ExpiresAt: k.ExpiresAt}).ValidateShape()
}
func (r BotKeyResult) Validate() error {
	if r.Key.Validate() != nil {
		return ErrInvalid
	}
	if r.Disclosure == "unavailable" {
		if r.Secret != "" {
			return ErrInvalid
		}
		return nil
	}
	if r.Disclosure != "first" || r.Key.Status != "active" || len(r.Secret) != 50 || !strings.HasPrefix(r.Secret, "lr_bot_") {
		return ErrInvalid
	}
	bytes, e := base64.RawURLEncoding.Strict().DecodeString(r.Secret[7:])
	if e != nil || len(bytes) != 32 {
		return ErrInvalid
	}
	return nil
}
