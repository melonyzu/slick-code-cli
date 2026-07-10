package types

import "encoding/json"

// Tool describes a function a model may call during a conversation.
type Tool struct {
	// Name uniquely identifies the tool within a request.
	Name string

	// Description tells the model what the tool does and when to use it.
	Description string

	// InputSchema is the JSON Schema of the tool's arguments.
	InputSchema json.RawMessage
}

// ToolCall is a model's request to invoke a tool.
type ToolCall struct {
	// ID correlates this call with its ToolResult.
	ID string

	// Name is the name of the tool being invoked.
	Name string

	// Input holds the call arguments as JSON matching the tool's
	// input schema.
	Input json.RawMessage
}

// ToolResult is the outcome of executing a tool call, returned to the
// model.
type ToolResult struct {
	// ToolCallID identifies the ToolCall this result answers.
	ToolCallID string

	// Content is the result payload presented to the model.
	Content string

	// IsError marks the result as a failed execution.
	IsError bool
}
