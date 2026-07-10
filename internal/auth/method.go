package auth

import (
	"fmt"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Method identifies an authentication strategy a provider supports.
type Method string

// Supported authentication methods.
const (
	// MethodAPIKey authenticates with a user-supplied API key.
	MethodAPIKey Method = "api_key"

	// MethodBrowserOAuth authenticates by completing an OAuth flow in
	// the user's browser.
	MethodBrowserOAuth Method = "browser_oauth"

	// MethodDeviceCode authenticates with the OAuth device code flow:
	// the user enters a short code on a verification page.
	MethodDeviceCode Method = "device_code"

	// MethodNone marks a provider usable without authentication, such
	// as a locally hosted model server.
	MethodNone Method = "none"
)

// Valid reports whether m is one of the supported methods.
func (m Method) Valid() bool {
	switch m {
	case MethodAPIKey, MethodBrowserOAuth, MethodDeviceCode, MethodNone:
		return true
	default:
		return false
	}
}

// ParseMethod converts a user-supplied string, such as a command-line
// flag value, into a Method.
func ParseMethod(s string) (Method, error) {
	m := Method(s)
	if !m.Valid() {
		return "", types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("unknown authentication method %q (want api_key, browser_oauth, device_code, or none)", s))
	}
	return m, nil
}
