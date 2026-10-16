// Package identityv1 defines stable actor identity and durable authority references.
package identityv1

type Principal struct {
	Subject string `json:"subject"`
	Domain  string `json:"domain"`
}

// AuthorityRef identifies the credential record and version, never its bearer value.
type AuthorityRef struct {
	Kind    string `json:"kind"`
	ID      string `json:"id"`
	Version uint64 `json:"version"`
}
type Identity struct {
	Principal Principal    `json:"principal"`
	Authority AuthorityRef `json:"authority"`
}

const (
	KindKratosSession          = "kratos_session"
	KindBotKey                 = "bot_key"
	KindGuestSession           = "guest_session"
	KindOAuthToken             = "oauth_token"
	KindService                = "service"
	KindDev                    = "dev"
	MaxIdentifierBytes         = 256
	MaxAuthorityVersion uint64 = 1<<63 - 1
)
