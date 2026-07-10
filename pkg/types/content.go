package types

// Part is one piece of a message's content. It is a sealed interface:
// the concrete types below are the complete set, so consumers can type
// switch over them exhaustively. Providers translate their own content
// formats into these parts and never leak provider-specific structures.
type Part interface {
	// part restricts implementations to this package.
	part()
}

// TextPart is plain text content.
type TextPart struct {
	Text string
}

// ReasoningPart is a model's intermediate reasoning, kept separate from
// its final answer text.
type ReasoningPart struct {
	Text string
}

// ImagePart is image content, by reference or inline.
type ImagePart struct {
	Image ImageRef
}

// FilePart is an attached file, by reference or inline.
type FilePart struct {
	File FileRef
}

// ToolCallPart is a model's request to invoke a tool.
type ToolCallPart struct {
	Call ToolCall
}

// ToolResultPart is the outcome of a tool call, sent back to the model.
type ToolResultPart struct {
	Result ToolResult
}

func (TextPart) part()       {}
func (ReasoningPart) part()  {}
func (ImagePart) part()      {}
func (FilePart) part()       {}
func (ToolCallPart) part()   {}
func (ToolResultPart) part() {}

// ImageRef locates image content: exactly one of URL or Data is set.
type ImageRef struct {
	// URL locates a remote image.
	URL string

	// Data holds inline image bytes.
	Data []byte

	// MediaType is the image's MIME type, such as "image/png".
	MediaType string
}

// FileRef locates file content: exactly one of Path, URL, or Data is set.
type FileRef struct {
	// Path locates a file on the local filesystem.
	Path string

	// URL locates a remote file.
	URL string

	// Data holds inline file bytes.
	Data []byte

	// Name is the file's display name, such as "main.go".
	Name string

	// MediaType is the file's MIME type, such as "text/plain".
	MediaType string
}
