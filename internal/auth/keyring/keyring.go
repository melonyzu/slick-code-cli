// Package keyring implements auth.Store on the operating system's
// native secure credential storage: Keychain on macOS, Credential
// Manager on Windows, and the Secret Service (e.g. GNOME Keyring,
// KWallet) on Linux. Credentials are stored as JSON payloads inside the
// vault and never touch a plain-text file.
package keyring

import (
	"encoding/json"
	"errors"
	"fmt"

	gokeyring "github.com/zalando/go-keyring"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// service is the vault entry namespace for all Slick Code credentials.
const service = "slickcode"

// Store is an auth.Store backed by the OS keyring.
type Store struct{}

// New returns a keyring-backed Store.
func New() *Store {
	return &Store{}
}

// Get implements auth.Store.
func (*Store) Get(provider types.Provider) (auth.Credential, error) {
	payload, err := gokeyring.Get(service, provider.String())
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return auth.Credential{}, auth.ErrNotFound
		}
		return auth.Credential{}, fmt.Errorf("keyring: read %s: %w", provider, err)
	}

	var cred auth.Credential
	if err := json.Unmarshal([]byte(payload), &cred); err != nil {
		return auth.Credential{}, fmt.Errorf("keyring: decode %s credential: %w", provider, err)
	}
	return cred, nil
}

// Set implements auth.Store.
func (*Store) Set(credential auth.Credential) error {
	payload, err := json.Marshal(credential)
	if err != nil {
		return fmt.Errorf("keyring: encode %s credential: %w", credential.Provider, err)
	}

	if err := gokeyring.Set(service, credential.Provider.String(), string(payload)); err != nil {
		return fmt.Errorf("keyring: write %s: %w", credential.Provider, err)
	}
	return nil
}

// Delete implements auth.Store.
func (*Store) Delete(provider types.Provider) error {
	if err := gokeyring.Delete(service, provider.String()); err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return auth.ErrNotFound
		}
		return fmt.Errorf("keyring: delete %s: %w", provider, err)
	}
	return nil
}
