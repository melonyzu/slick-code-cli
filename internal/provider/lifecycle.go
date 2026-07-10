package provider

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/melonyzu/slick-code-cli/internal/auth"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// The interfaces below are optional lifecycle hooks a provider may
// implement, discovered by type assertion like the operation contracts
// in contracts.go. A provider with no setup or teardown needs none of
// them.

// Activator is implemented by providers that need setup before serving
// requests, such as building an API client from a credential.
type Activator interface {
	// Activate prepares the provider for use. cred is the zero
	// Credential for providers activated without authentication.
	Activate(ctx context.Context, cred auth.Credential) error
}

// Deactivator is implemented by providers that hold resources needing
// release when the provider is taken out of use.
type Deactivator interface {
	// Deactivate releases the provider's resources.
	Deactivate(ctx context.Context) error
}

// HealthChecker is implemented by providers that can verify they are
// able to serve requests.
type HealthChecker interface {
	// CheckHealth reports an error when the provider cannot currently
	// serve requests.
	CheckHealth(ctx context.Context) error
}

// Lifecycle manages providers through their runtime states: registered,
// active, deactivated. Activation resolves the provider's session
// through the auth manager — validating and refreshing it as needed —
// and runs the provider's optional hooks. Lifecycle contains no
// provider-specific logic.
type Lifecycle struct {
	registry *Registry
	sessions *auth.Manager
	logger   *slog.Logger

	mu     sync.Mutex
	active map[types.Provider]Provider
}

// NewLifecycle returns a Lifecycle over the given registry and sessions.
func NewLifecycle(registry *Registry, sessions *auth.Manager, logger *slog.Logger) *Lifecycle {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Lifecycle{
		registry: registry,
		sessions: sessions,
		logger:   logger,
		active:   make(map[types.Provider]Provider),
	}
}

// Activate brings the named provider into service and returns it.
// Activating an already-active provider is a no-op. When the provider
// requires authentication, Activate resolves its session, refreshing an
// expired credential if the provider implements auth.Refresher and
// failing with a typed authentication error otherwise.
func (l *Lifecycle) Activate(ctx context.Context, name types.Provider) (Provider, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if p, ok := l.active[name]; ok {
		return p, nil
	}

	p, err := l.registry.Get(name)
	if err != nil {
		return nil, err
	}

	var cred auth.Credential
	if a, ok := p.(Authenticator); ok {
		cred, err = l.resolveCredential(ctx, name, p, a)
		if err != nil {
			return nil, err
		}
	}

	if act, ok := p.(Activator); ok {
		if err := act.Activate(ctx, cred); err != nil {
			return nil, err
		}
	}

	l.active[name] = p
	l.logger.Info("provider activated", "provider", name)
	return p, nil
}

// Deactivate takes the named provider out of service, releasing its
// resources if it implements Deactivator.
func (l *Lifecycle) Deactivate(ctx context.Context, name types.Provider) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	p, ok := l.active[name]
	if !ok {
		return types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("provider %q is not active", name))
	}

	if d, ok := p.(Deactivator); ok {
		if err := d.Deactivate(ctx); err != nil {
			return err
		}
	}

	delete(l.active, name)
	l.logger.Info("provider deactivated", "provider", name)
	return nil
}

// Active returns the names of the currently active providers, in no
// particular order.
func (l *Lifecycle) Active() []types.Provider {
	l.mu.Lock()
	defer l.mu.Unlock()

	names := make([]types.Provider, 0, len(l.active))
	for name := range l.active {
		names = append(names, name)
	}
	return names
}

// CheckHealth reports whether the named active provider can serve
// requests. Providers that do not implement HealthChecker are
// considered healthy while active.
func (l *Lifecycle) CheckHealth(ctx context.Context, name types.Provider) error {
	l.mu.Lock()
	p, ok := l.active[name]
	l.mu.Unlock()

	if !ok {
		return types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("provider %q is not active", name))
	}

	if h, ok := p.(HealthChecker); ok {
		return h.CheckHealth(ctx)
	}
	return nil
}

// resolveCredential produces the credential to activate p with: the
// stored session's, a refreshed one when expired and refreshable, or
// the zero Credential for providers that permit unauthenticated use.
func (l *Lifecycle) resolveCredential(ctx context.Context, name types.Provider, p Provider, a Authenticator) (auth.Credential, error) {
	sess, err := l.sessions.Session(ctx, name)
	if err != nil {
		if errors.Is(err, auth.ErrNotFound) && !RequiresAuth(a.AuthMethods()) {
			return auth.Credential{}, nil
		}
		return auth.Credential{}, err
	}

	if sess.Valid(time.Now()) {
		return sess.Credential, nil
	}

	if r, ok := p.(auth.Refresher); ok {
		return l.sessions.Refresh(ctx, name, r)
	}

	return auth.Credential{}, &types.Error{
		Kind:     types.ErrorKindAuthentication,
		Provider: name,
		Message:  "session expired (run: slickcode auth login " + name.String() + ")",
	}
}
