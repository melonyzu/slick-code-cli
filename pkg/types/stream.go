package types

// StreamEvent is one increment of a streamed response. It is a sealed
// interface: the concrete types below are the complete set, so consumers
// can type switch over them exhaustively.
type StreamEvent interface {
	// streamEvent restricts implementations to this package.
	streamEvent()
}

// TextEvent carries a fragment of the model's answer text.
type TextEvent struct {
	Text string
}

// ReasoningEvent carries a fragment of the model's intermediate
// reasoning.
type ReasoningEvent struct {
	Text string
}

// ToolCallEvent carries a complete tool call the model wants executed.
type ToolCallEvent struct {
	Call ToolCall
}

// DoneEvent terminates a stream, carrying the assembled final response.
type DoneEvent struct {
	Response Response
}

func (TextEvent) streamEvent()      {}
func (ReasoningEvent) streamEvent() {}
func (ToolCallEvent) streamEvent()  {}
func (DoneEvent) streamEvent()      {}
