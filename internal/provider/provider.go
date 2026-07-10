// Package provider defines the extension point through which Slick Code
// integrates with AI coding providers. Each provider (Anthropic, OpenAI,
// Google, Ollama, OpenRouter, ...) implements the Provider interface — and
// whichever optional capability interfaces from contracts.go it supports —
// in its own subpackage, translating between its API and the domain model
// in pkg/types. This package knows nothing about any specific provider.
package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Provider is the base interface implemented by every supported AI coding
// provider. Operations beyond model discovery are expressed as the
// optional capability interfaces in contracts.go.
type Provider interface {
	// Name returns the provider's identifier, matching one of the
	// constants in pkg/types.
	Name() types.Provider

	// Models returns the models this provider offers, including each
	// model's capabilities.
	Models(ctx context.Context) ([]types.Model, error)
}

// Registry holds the set of providers available to a running application.
// Providers are registered explicitly during application bootstrap; there
// is no package-level global state, so each application instance can hold
// its own independent set (useful for tests).
type Registry struct {
	mu        sync.RWMutex
	providers map[types.Provider]Provider
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{providers: make(map[types.Provider]Provider)}
}

// Register adds a provider to the registry. It returns an error if a
// provider with the same name is already registered.
func (r *Registry) Register(p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := p.Name()
	if _, exists := r.providers[name]; exists {
		return types.NewError(types.ErrorKindInternal,
			fmt.Sprintf("provider %q is already registered", name))
	}
	r.providers[name] = p
	return nil
}

// Get returns the registered provider with the given name.
func (r *Registry) Get(name types.Provider) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("provider %q is not registered", name))
	}
	return p, nil
}

// List returns the names of all registered providers, sorted
// alphabetically. It is the mechanism by which the rest of the
// application discovers what providers are available.
func (r *Registry) List() []types.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]types.Provider, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	return names
}
