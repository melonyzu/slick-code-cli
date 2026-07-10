package core

import (
	"context"
	"fmt"

	"github.com/melonyzu/slick-code-cli/internal/provider"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// EnsureActive brings the named provider into service, restoring an
// existing session when one is stored and automatically running the
// provider's preferred authentication flow when none is. This is the
// path behind "just run slickcode": the user never has to invoke a
// login command explicitly.
func (a *App) EnsureActive(ctx context.Context, name types.Provider) (provider.Provider, error) {
	p, err := a.Lifecycle.Activate(ctx, name)
	if err == nil {
		return p, nil
	}
	if types.KindOf(err) != types.ErrorKindAuthentication {
		return nil, err
	}

	authn, cerr := provider.Capability[provider.Authenticator](a.Providers, name)
	if cerr != nil {
		return nil, err // the original authentication error is the useful one
	}

	methods := authn.AuthMethods()
	if len(methods) == 0 {
		return nil, err
	}

	flow, ferr := authn.NewFlow(methods[0])
	if ferr != nil {
		return nil, ferr
	}

	a.Terminal.Notify(fmt.Sprintf("Authentication required for %s.", name))
	if _, lerr := a.Auth.Login(ctx, name, flow, a.Terminal); lerr != nil {
		return nil, lerr
	}

	return a.Lifecycle.Activate(ctx, name)
}
