package types

import "strings"

// Role identifies the author of a message exchanged with a provider.
type Role string

// Supported message roles.
const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a conversation. Its content is an ordered
// sequence of parts, allowing one message to mix text, images, files,
// tool calls, and tool results.
type Message struct {
	Role     Role
	Parts    []Part
	Metadata Metadata
}

// NewTextMessage returns a Message containing a single text part.
func NewTextMessage(role Role, text string) Message {
	return Message{Role: role, Parts: []Part{TextPart{Text: text}}}
}

// Text returns the concatenated text parts of the message, ignoring all
// other part kinds.
func (m Message) Text() string {
	var b strings.Builder
	for _, p := range m.Parts {
		if t, ok := p.(TextPart); ok {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}
