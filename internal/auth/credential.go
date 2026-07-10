package auth

import (
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Credential is the stored outcome of a successful authentication. One
// struct covers every method: an API key uses only Secret, an OAuth
// grant additionally carries RefreshToken and ExpiresAt, and MethodNone
// carries no secret material at all.
type Credential struct {
	// Provider is the provider this credential authenticates with.
	Provider types.Provider `json:"provider"`

	// Method is the authentication method that produced the credential.
	Method Method `json:"method"`

	// Secret is the API key or access token.
	Secret Secret `json:"secret,omitempty"`

	// RefreshToken renews Secret when it expires, for methods that
	// support refresh.
	RefreshToken Secret `json:"refresh_token,omitempty"`

	// ExpiresAt is when Secret stops being accepted. The zero value
	// means the credential does not expire.
	ExpiresAt time.Time `json:"expires_at,omitzero"`

	// Metadata carries non-sensitive method-specific values, such as a
	// token type. Secrets must never be placed here.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Expired reports whether the credential's secret has expired as of now.
// Credentials without an expiry never expire.
func (c Credential) Expired(now time.Time) bool {
	return !c.ExpiresAt.IsZero() && now.After(c.ExpiresAt)
}
