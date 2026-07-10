package tool

import (
	"fmt"
	"sort"
	"sync"

	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// Registry holds the set of tools available to a running application.
// Tools are registered explicitly during application bootstrap; there is
// no package-level global state, so each application instance can hold
// its own independent set (useful for tests).
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. It returns an error if the
// tool's name is empty or already registered.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Definition().Name
	if name == "" {
		return types.NewError(types.ErrorKindInternal,
			"tool has an empty name")
	}
	if _, exists := r.tools[name]; exists {
		return types.NewError(types.ErrorKindInternal,
			fmt.Sprintf("tool %q is already registered", name))
	}
	r.tools[name] = t
	return nil
}

// Get returns the registered tool with the given name.
func (r *Registry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	if !ok {
		return nil, types.NewError(types.ErrorKindValidation,
			fmt.Sprintf("tool %q is not registered", name))
	}
	return t, nil
}

// List returns the names of all registered tools, sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Definitions returns every registered tool's definition, sorted by
// name, in the form a types.Request carries to the provider.
func (r *Registry) Definitions() []types.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]types.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs
}
