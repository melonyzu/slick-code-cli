package types

// StopReason explains why a model stopped generating.
type StopReason string

// Known stop reasons.
const (
	// StopReasonEndTurn means the model finished its answer naturally.
	StopReasonEndTurn StopReason = "end_turn"

	// StopReasonMaxTokens means generation hit the token limit.
	StopReasonMaxTokens StopReason = "max_tokens"

	// StopReasonToolUse means the model stopped to invoke tools.
	StopReasonToolUse StopReason = "tool_use"
)

// Response is a provider-agnostic completion response, translated by a
// provider from its own API's response format.
type Response struct {
	// Message is the model's reply.
	Message Message

	// Model is the ID of the model that produced the response.
	Model string

	// StopReason explains why generation ended.
	StopReason StopReason

	// Usage reports the request's token consumption.
	Usage Usage

	// Metadata carries optional annotations on the response.
	Metadata Metadata
}
