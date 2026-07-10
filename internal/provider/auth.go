package provider

import (
	"github.com/melonyzu/slick-code-cli/internal/auth"
)

// Authenticator is the optional contract through which a provider
// advertises and performs authentication. The runtime discovers it by
// type assertion (see Capability) and drives whatever flow the provider
// returns via auth.Manager — no provider-specific authentication logic
// exists outside the provider's own subpackage.
//
// Providers whose credentials expire additionally implement
// auth.Refresher, discovered the same way.
type Authenticator interface {
	// AuthMethods returns the authentication methods the provider
	// supports, in order of preference. A provider that can operate
	// unauthenticated includes auth.MethodNone.
	AuthMethods() []auth.Method

	// NewFlow returns a fresh flow performing the given method. It
	// returns an error of kind types.ErrorKindUnsupportedCapability
	// for methods not present in AuthMethods.
	NewFlow(method auth.Method) (auth.Flow, error)
}

// RequiresAuth reports whether a provider advertising these methods
// needs a session before it can be activated. A provider that lists
// auth.MethodNone can always operate without one.
func RequiresAuth(methods []auth.Method) bool {
	for _, m := range methods {
		if m == auth.MethodNone {
			return false
		}
	}
	return len(methods) > 0
}
