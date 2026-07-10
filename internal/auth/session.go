package auth

import (
	"time"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Session is the authentication state for one provider: its stored
// credential and derived validity. It is distinct from provider runtime
// state (activation), which internal/provider owns.
type Session struct {
	// Credential is the stored credential backing the session.
	Credential Credential
}

// Provider returns the provider the session authenticates with.
func (s Session) Provider() types.Provider { return s.Credential.Provider }

// Method returns the authentication method that created the session.
func (s Session) Method() Method { return s.Credential.Method }

// Valid reports whether the session's credential is usable as of now.
func (s Session) Valid(now time.Time) bool {
	return !s.Credential.Expired(now)
}
