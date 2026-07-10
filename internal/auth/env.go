package auth

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// ErrReadOnly is returned by stores that cannot persist or remove
// credentials, such as EnvStore. Layered uses it to fall through to the
// next writable layer.
var ErrReadOnly = errors.New("auth: store is read-only")

// EnvStore is a read-only Store that resolves credentials from
// environment variables of the form SLICKCODE_<PROVIDER>_API_KEY, e.g.
// SLICKCODE_ANTHROPIC_API_KEY. It exists for CI and other environments
// where an interactive login or an OS keyring is unavailable; the
// variable's value is treated as an API-key credential.
type EnvStore struct{}

// NewEnvStore returns an EnvStore.
func NewEnvStore() *EnvStore {
	return &EnvStore{}
}

// EnvVar returns the environment variable EnvStore reads for the given
// provider.
func EnvVar(provider types.Provider) string {
	return fmt.Sprintf("SLICKCODE_%s_API_KEY", strings.ToUpper(provider.String()))
}

// Get implements Store.
func (*EnvStore) Get(provider types.Provider) (Credential, error) {
	value := os.Getenv(EnvVar(provider))
	if value == "" {
		return Credential{}, ErrNotFound
	}
	return Credential{
		Provider: provider,
		Method:   MethodAPIKey,
		Secret:   Secret(value),
	}, nil
}

// Set implements Store; environment credentials cannot be written.
func (*EnvStore) Set(Credential) error {
	return ErrReadOnly
}

// Delete implements Store. It returns a descriptive error when the
// variable is set — the credential exists but only the user can remove
// it — and ErrNotFound otherwise.
func (*EnvStore) Delete(provider types.Provider) error {
	name := EnvVar(provider)
	if os.Getenv(name) == "" {
		return ErrNotFound
	}
	return &types.Error{
		Kind:     types.ErrorKindValidation,
		Provider: provider,
		Message:  fmt.Sprintf("credential comes from the %s environment variable; unset it to log out", name),
		Err:      ErrReadOnly,
	}
}
