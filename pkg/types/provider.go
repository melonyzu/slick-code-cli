// Package types defines the stable, public data types shared across Slick
// Code's command-line interface and provider implementations. Types in this
// package form part of the project's public API and changes to them must
// follow semantic versioning.
package types

// Provider identifies a supported AI coding provider.
type Provider string

// Supported providers.
const (
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGoogle     Provider = "google"
	ProviderOllama     Provider = "ollama"
	ProviderOpenRouter Provider = "openrouter"
)

// String returns the provider name as used in configuration files and
// command-line flags.
func (p Provider) String() string {
	return string(p)
}

// Valid reports whether p is one of the supported providers.
func (p Provider) Valid() bool {
	switch p {
	case ProviderAnthropic, ProviderOpenAI, ProviderGoogle, ProviderOllama, ProviderOpenRouter:
		return true
	default:
		return false
	}
}
