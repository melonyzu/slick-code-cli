package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Manager drives the authentication lifecycle: login, logout, session
// discovery, validation, and refresh. It owns the choreography for each
// authentication method, so the login experience is identical no matter
// which provider supplied the flow. It never logs or displays secret
// material.
type Manager struct {
	store  Store
	logger *slog.Logger
}

// NewManager returns a Manager persisting credentials to store.
func NewManager(store Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Manager{store: store, logger: logger}
}

// Login drives flow to completion, interacting with the user through
// ui, and persists the resulting credential for name.
func (m *Manager) Login(ctx context.Context, name types.Provider, flow Flow, ui Prompter) (Credential, error) {
	var (
		cred Credential
		err  error
	)

	switch f := flow.(type) {
	case APIKeyFlow:
		var key string
		key, err = ui.PromptSecret(fmt.Sprintf("Paste your %s API key", name))
		if err == nil {
			cred, err = f.Exchange(ctx, key)
		}

	case BrowserFlow:
		var url string
		url, err = f.Start(ctx)
		if err == nil {
			ui.Notify("Open this URL in your browser to continue:\n\n  " + url + "\n")
			cred, err = f.Wait(ctx)
		}

	case DeviceCodeFlow:
		var authz DeviceAuthorization
		authz, err = f.Start(ctx)
		if err == nil {
			ui.Notify(fmt.Sprintf("Visit %s and enter the code %s to continue.",
				authz.VerificationURL, authz.UserCode))
			cred, err = f.Wait(ctx)
		}

	case NoneFlow:
		// Nothing to exchange: record that the provider is used
		// without authentication.

	default:
		return Credential{}, types.NewError(types.ErrorKindInternal,
			fmt.Sprintf("no login choreography for flow %T", flow))
	}

	if err != nil {
		return Credential{}, err
	}

	cred.Provider = name
	cred.Method = flow.Method()

	if err := m.store.Set(cred); err != nil {
		return Credential{}, types.WrapError(types.ErrorKindInternal,
			"store credential", err)
	}

	m.logger.Info("logged in", "provider", name, "method", cred.Method)
	return cred, nil
}

// Logout removes the stored credential for name.
func (m *Manager) Logout(ctx context.Context, name types.Provider) error {
	if err := m.store.Delete(name); err != nil {
		if errors.Is(err, ErrNotFound) {
			return m.notLoggedIn(name)
		}
		return types.WrapError(types.ErrorKindInternal, "delete credential", err)
	}

	m.logger.Info("logged out", "provider", name)
	return nil
}

// Session returns the authentication state for name, or an
// authentication error wrapping ErrNotFound if the user is not logged
// in. The caller decides whether an invalid (expired) session is
// acceptable; see Session.Valid.
func (m *Manager) Session(ctx context.Context, name types.Provider) (Session, error) {
	cred, err := m.store.Get(name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Session{}, m.notLoggedIn(name)
		}
		return Session{}, types.WrapError(types.ErrorKindInternal, "read credential", err)
	}
	return Session{Credential: cred}, nil
}

// Sessions returns the sessions that exist among the given providers,
// preserving order. Providers the user is not logged in to are simply
// omitted; any other storage failure is returned.
func (m *Manager) Sessions(ctx context.Context, names []types.Provider) ([]Session, error) {
	sessions := make([]Session, 0, len(names))
	for _, name := range names {
		sess, err := m.Session(ctx, name)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// Refresh renews the stored credential for name using r and persists
// the result.
func (m *Manager) Refresh(ctx context.Context, name types.Provider, r Refresher) (Credential, error) {
	sess, err := m.Session(ctx, name)
	if err != nil {
		return Credential{}, err
	}

	cred, err := r.Refresh(ctx, sess.Credential)
	if err != nil {
		return Credential{}, err
	}

	cred.Provider = name
	if err := m.store.Set(cred); err != nil {
		return Credential{}, types.WrapError(types.ErrorKindInternal,
			"store refreshed credential", err)
	}

	m.logger.Info("session refreshed", "provider", name)
	return cred, nil
}

// notLoggedIn returns the typed error for a missing session, keeping
// ErrNotFound in the chain so callers can distinguish "not logged in"
// from other authentication failures with errors.Is.
func (m *Manager) notLoggedIn(name types.Provider) error {
	return &types.Error{
		Kind:     types.ErrorKindAuthentication,
		Provider: name,
		Message:  "not logged in (run: slickcode auth login " + name.String() + ")",
		Err:      ErrNotFound,
	}
}
