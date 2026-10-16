package accessv1

import (
	"encoding/base64"
	"errors"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrInvalid = errors.New("invalid hierarchy request")
var ErrConflict = errors.New("hierarchy conflict")

type Organization struct {
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
}
type Workspace struct {
	OrganizationID string `json:"organizationId"`
	WorkspaceID    string `json:"workspaceId"`
	Name           string `json:"name"`
}
type Team struct {
	OrganizationID string `json:"organizationId"`
	TeamID         string `json:"teamId"`
	Name           string `json:"name"`
}
type Project struct {
	OrganizationID string `json:"organizationId"`
	WorkspaceID    string `json:"workspaceId"`
	ProjectID      string `json:"projectId"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	OwnerID        string `json:"ownerId,omitempty"`
	TeamID         string `json:"teamId,omitempty"`
}
type Membership struct {
	OrganizationID string `json:"organizationId"`
	HumanID        string `json:"humanId"`
	Role           string `json:"role"`
	TeamID         string `json:"teamId,omitempty"`
}
type Invitation struct {
	OrganizationID string    `json:"organizationId"`
	InvitationID   string    `json:"invitationId"`
	HumanID        string    `json:"humanId"`
	Role           string    `json:"role"`
	ExpiresAt      time.Time `json:"expiresAt"`
	State          string    `json:"state"`
}
type IssuedInvitation struct {
	Invitation Invitation `json:"invitation"`
	Secret     string     `json:"secret"`
}
type NameRequest struct {
	Name string `json:"name"`
}
type ProjectRequest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	OwnerID string `json:"ownerId,omitempty"`
	TeamID  string `json:"teamId,omitempty"`
}
type MembershipRequest struct {
	HumanID string `json:"humanId"`
	Role    string `json:"role"`
}
type InvitationRequest struct {
	HumanID   string    `json:"humanId"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type AcceptInvitationRequest struct {
	Secret string `json:"secret"`
}
type Result struct {
	Applied bool `json:"applied"`
}
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}
type PageRequest struct {
	Limit  int
	Cursor string
}

func ValidName(s string) bool {
	if s == "" || len(s) > 128 || !utf8.ValidString(s) || !identityv1.ValidIdentifier(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (r NameRequest) Validate() error {
	if !ValidName(r.Name) {
		return ErrInvalid
	}
	return nil
}
func (r ProjectRequest) Validate() error {
	if !ValidName(r.Name) {
		return ErrInvalid
	}
	if r.Kind == "personal" && identityv1.ValidIdentifier(r.OwnerID) && r.TeamID == "" {
		return nil
	}
	if r.Kind == "team" && identityv1.ValidIdentifier(r.TeamID) && r.OwnerID == "" {
		return nil
	}
	return ErrInvalid
}
func (r MembershipRequest) Validate() error {
	if !identityv1.ValidIdentifier(r.HumanID) || (r.Role != "member" && r.Role != "owner") {
		return ErrInvalid
	}
	return nil
}
func (r InvitationRequest) Validate(now time.Time) error {
	if (MembershipRequest{r.HumanID, r.Role}).Validate() != nil || !r.ExpiresAt.After(now) || r.ExpiresAt.After(now.Add(7*24*time.Hour)) {
		return ErrInvalid
	}
	return nil
}
func (r AcceptInvitationRequest) Validate() error {
	b, e := base64.RawURLEncoding.DecodeString(r.Secret)
	if e != nil || len(b) != 32 || base64.RawURLEncoding.EncodeToString(b) != r.Secret {
		return ErrInvalid
	}
	return nil
}
func (r PageRequest) Validate() error {
	if r.Limit < 1 || r.Limit > 100 || len(r.Cursor) > 2048 {
		return ErrInvalid
	}
	return nil
}
