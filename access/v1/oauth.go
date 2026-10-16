package accessv1

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"sort"
)

const OAuthScopeLimit = 6
const OAuthChallengeLimit = 8192

var oauthStatePattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// OAuthScopes is a closed restriction vocabulary. Protocol offline access never
// supplies an application operation or an underlying permission relation.
var OAuthScopes = []string{"leafrun.generate", "leafrun.status", "leafrun.source.read", "leafrun.resource.read", "leafrun.download", "offline_access"}

func OAuthCeiling(scopes []string) (*CredentialCeiling, error) {
	if len(scopes) == 0 || len(scopes) > OAuthScopeLimit {
		return nil, ErrInvalid
	}
	c := &CredentialCeiling{Operations: []string{}}
	seen := map[string]bool{}
	for _, s := range scopes {
		if seen[s] {
			return nil, ErrInvalid
		}
		seen[s] = true
		switch s {
		case "leafrun.generate":
			c.Operations = append(c.Operations, "generation.create", "generation.execute", "generation.publish", "generation.cancel")
		case "leafrun.status":
			c.Operations = append(c.Operations, "generation.read")
		case "leafrun.source.read":
			c.Operations = append(c.Operations, "source.read")
		case "leafrun.resource.read":
			c.Operations = append(c.Operations, "resource.read")
		case "leafrun.download":
			c.Operations = append(c.Operations, "artifact.read", "download")
		case "offline_access":
		default:
			return nil, ErrInvalid
		}
	}
	sort.Strings(c.Operations)
	return c, nil
}

// OAuthConsent is safe public metadata. Credentials and provider challenges are
// deliberately absent; the browser flow owns its one-use form state separately.
type OAuthConsent struct {
	ID       string   `json:"id"`
	ClientID string   `json:"clientId"`
	Scopes   []string `json:"scopes"`
	Audience []string `json:"audience"`
}

// OAuthBrowserForm is an HTML form contract, never a JSON consent API. State and
// CSRF identify an owned one-use browser transition; they grant no authority.
// Requested scopes are supplied by the verified provider request, not the form.
type OAuthBrowserForm struct {
	State  string   `json:"state"`
	CSRF   string   `json:"csrf"`
	Action string   `json:"action"`
	Scopes []string `json:"scope,omitempty"`
}

func (f OAuthBrowserForm) Validate(purpose string, requested []string) error {
	csrf, err := base64.RawURLEncoding.Strict().DecodeString(f.CSRF)
	if !oauthStatePattern.MatchString(f.State) || len(f.CSRF) != 43 || err != nil || len(csrf) != 32 {
		return ErrInvalid
	}
	if purpose == "login" {
		if f.Action != "accept" || len(f.Scopes) != 0 {
			return ErrInvalid
		}
		return nil
	}
	if purpose != "consent" || (f.Action != "accept" && f.Action != "deny") || len(f.Scopes) > OAuthScopeLimit {
		return ErrInvalid
	}
	if _, err := OAuthCeiling(requested); err != nil {
		return ErrInvalid
	}
	if len(f.Scopes) == 0 {
		if f.Action == "deny" {
			return nil
		}
		return ErrInvalid
	}
	if _, err := OAuthCeiling(f.Scopes); err != nil {
		return ErrInvalid
	}
	allowed := make(map[string]bool, len(requested))
	for _, scope := range requested {
		allowed[scope] = true
	}
	for _, scope := range f.Scopes {
		if !allowed[scope] {
			return ErrInvalid
		}
	}
	return nil
}

// ParseOAuthBrowserForm rejects extra/duplicate scalar keys and preserves no
// selected scope on denial. HTTP callers must separately bound/read the body,
// require form content type, reject query input, and verify live browser bindings.
func ParseOAuthBrowserForm(values url.Values, purpose string, requested []string) (OAuthBrowserForm, error) {
	for key, entries := range values {
		switch key {
		case "state", "csrf", "action":
			if len(entries) != 1 {
				return OAuthBrowserForm{}, ErrInvalid
			}
		case "scope":
			if purpose != "consent" || len(entries) == 0 || len(entries) > OAuthScopeLimit {
				return OAuthBrowserForm{}, ErrInvalid
			}
		default:
			return OAuthBrowserForm{}, ErrInvalid
		}
	}
	f := OAuthBrowserForm{State: values.Get("state"), CSRF: values.Get("csrf"), Action: values.Get("action"), Scopes: append([]string(nil), values["scope"]...)}
	if f.Validate(purpose, requested) != nil {
		return OAuthBrowserForm{}, ErrInvalid
	}
	if f.Action == "deny" {
		f.Scopes = nil
	}
	return f, nil
}
