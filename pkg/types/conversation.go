package types

// Conversation is an ordered exchange of messages. It is pure data;
// runtime management of an active conversation lives in
// internal/session.
type Conversation struct {
	// ID uniquely identifies the conversation, if it has been persisted.
	ID string

	// Title is a human-readable label for the conversation.
	Title string

	// Messages holds the conversation's turns in chronological order.
	Messages []Message

	// Metadata carries optional annotations on the conversation.
	Metadata Metadata
}
