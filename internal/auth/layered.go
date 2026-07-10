package auth

import (
	"errors"
	"log/slog"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Layered composes credential stores. Reads return the credential from
// the first layer that has one; writes go to the first layer that
// accepts them, falling through when a layer is read-only or failing —
// so a broken OS keyring (e.g. a headless machine without a secret
// service) degrades to the next layer instead of making login
// impossible.
type Layered struct {
	layers []Store
	logger *slog.Logger
}

// NewLayered returns a Layered store over the given layers, ordered
// from highest read precedence to lowest.
func NewLayered(logger *slog.Logger, layers ...Store) *Layered {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Layered{layers: layers, logger: logger}
}

// Get implements Store: the first layer holding a credential wins.
func (l *Layered) Get(provider types.Provider) (Credential, error) {
	for _, layer := range l.layers {
		cred, err := layer.Get(provider)
		if err == nil {
			return cred, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return Credential{}, err
		}
	}
	return Credential{}, ErrNotFound
}

// Set implements Store: the first layer that accepts the write wins.
// Failed layers are logged and skipped, so the credential survives at
// reduced durability rather than not at all.
func (l *Layered) Set(credential Credential) error {
	var lastErr error
	for _, layer := range l.layers {
		err := layer.Set(credential)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrReadOnly) {
			l.logger.Warn("credential store rejected write; trying next layer",
				"provider", credential.Provider, "error", err)
		}
		lastErr = err
	}
	return lastErr
}

// Delete implements Store: the credential is removed from every layer
// holding it. A layer that holds the credential but cannot remove it
// (such as an environment variable) surfaces its error.
func (l *Layered) Delete(provider types.Provider) error {
	deleted := false
	for _, layer := range l.layers {
		err := layer.Delete(provider)
		switch {
		case err == nil:
			deleted = true
		case errors.Is(err, ErrNotFound):
			// This layer never had it; keep going.
		default:
			return err
		}
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}
