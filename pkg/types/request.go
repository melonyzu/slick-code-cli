package types

// Request is a provider-agnostic completion request. Providers translate
// it into their own API's request format; nothing outside a provider
// implementation ever sees that format.
type Request struct {
	// Model is the ID of the model to use.
	Model string

	// Messages is the conversation to complete, in chronological order.
	Messages []Message

	// Tools lists the tools the model may call. Empty means tool
	// calling is not offered.
	Tools []Tool

	// MaxTokens caps the response length. Zero means the provider's
	// default.
	MaxTokens int

	// Metadata carries optional annotations on the request.
	Metadata Metadata
}
