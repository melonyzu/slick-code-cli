package auth

import (
	"errors"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// ErrNotFound is returned (possibly wrapped) by Store implementations
// when no credential is stored for the requested provider. Callers test
// for it with errors.Is.
var ErrNotFound = errors.New("auth: credential not found")

// Store persists and retrieves credentials. Implementations must keep
// credentials out of plain-text files: the default implementation is the
// OS-native keyring (see the keyring subpackage), and alternatives such
// as an encrypted file store can be added behind this same interface.
type Store interface {
	// Get returns the credential stored for the given provider, or an
	// error wrapping ErrNotFound if there is none.
	Get(provider types.Provider) (Credential, error)

	// Set stores a credential for its provider, replacing any existing
	// one.
	Set(credential Credential) error

	// Delete removes the stored credential for the given provider, or
	// returns an error wrapping ErrNotFound if there is none.
	Delete(provider types.Provider) error
}
