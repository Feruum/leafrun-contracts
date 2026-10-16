package identityv1

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidIdentifier checks transport syntax only; provenance belongs to the owner.
func ValidIdentifier(value string) bool {
	if value == "" || len(value) > MaxIdentifierBytes || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func (p Principal) Validate() error {
	if !ValidIdentifier(p.Subject) || !ValidIdentifier(p.Domain) {
		return errors.New("invalid principal")
	}
	return nil
}
func (a AuthorityRef) Validate() error {
	switch a.Kind {
	case KindKratosSession, KindBotKey, KindGuestSession, KindOAuthToken, KindService, KindDev:
	default:
		return errors.New("invalid authority kind")
	}
	if !ValidIdentifier(a.ID) {
		return errors.New("invalid authority ID")
	}
	if a.Version == 0 || a.Version > MaxAuthorityVersion {
		return errors.New("invalid authority version")
	}
	return nil
}
func (i Identity) Validate() error {
	if err := i.Principal.Validate(); err != nil {
		return err
	}
	return i.Authority.Validate()
}
