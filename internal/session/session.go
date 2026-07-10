// Package session manages the in-memory state of an active conversation
// during a single Slick Code run. The conversation data itself is the
// domain type types.Conversation; Session owns its mutation.
package session

import "github.com/melonyzu/slick-code-cli/pkg/types"

// Session tracks an active conversation.
type Session struct {
	conv types.Conversation
}

// New returns a Session holding an empty conversation.
func New() *Session {
	return &Session{}
}

// Append adds a message to the end of the conversation.
func (s *Session) Append(msg types.Message) {
	s.conv.Messages = append(s.conv.Messages, msg)
}

// Messages returns the conversation history in chronological order.
func (s *Session) Messages() []types.Message {
	return s.conv.Messages
}

// Conversation returns a snapshot of the underlying conversation.
func (s *Session) Conversation() types.Conversation {
	return s.conv
}
