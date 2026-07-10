package types

// Model describes a model offered by a provider, including which
// capabilities it supports. Capabilities are attached to models rather
// than providers because they vary between a provider's models.
type Model struct {
	// ID is the provider's identifier for the model, as sent in
	// requests, e.g. "claude-sonnet-5".
	ID string

	// Name is the model's human-readable display name.
	Name string

	// Provider is the provider that serves this model.
	Provider Provider

	// Capabilities is the set of features this model supports.
	Capabilities CapabilitySet
}
