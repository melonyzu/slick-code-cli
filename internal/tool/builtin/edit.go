package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/melonyzu/slick-code-cli/internal/edit"
	"github.com/melonyzu/slick-code-cli/internal/tool"
	"github.com/melonyzu/slick-code-cli/pkg/types"
)

// The editing tools adapt the edit.Engine into the tool framework. Each
// tool parses its arguments, confines the path to the working
// directory, and hands a fully resolved edit.Request to the shared
// engine — permissions, dry-run, and timeouts stay with the tool
// Manager, and every provider requests edits through these tools.

// editArgs is the superset of arguments across the editing tools; each
// tool documents the subset it reads in its input schema.
type editArgs struct {
	Path        string `json:"path"`
	NewPath     string `json:"new_path"`
	Content     string `json:"content"`
	Target      string `json:"target"`
	Replacement string `json:"replacement"`
	All         bool   `json:"all"`
	Line        int    `json:"line"`
	Text        string `json:"text"`
	BaseHash    string `json:"base_hash"`
	Preview     bool   `json:"preview"`
}

// parseEditArgs decodes input and requires a path.
func parseEditArgs(name string, input json.RawMessage) (editArgs, error) {
	var args editArgs
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return args, types.WrapError(types.ErrorKindValidation, name+": invalid input", err)
		}
	}
	if args.Path == "" {
		return args, types.NewError(types.ErrorKindValidation, name+": path is required")
	}
	return args, nil
}

// runEdit previews or applies req through the engine.
func runEdit(ctx context.Context, engine *edit.Engine, req edit.Request, preview bool) (*edit.Result, error) {
	if preview {
		return engine.Preview(ctx, req)
	}
	return engine.Apply(ctx, req)
}

// editContent renders an edit result for the model: the summary, the
// base hash a preview can be re-applied against, and the diff.
func editContent(res *edit.Result, summary string) string {
	var b strings.Builder
	if !res.Applied {
		b.WriteString("preview (no changes applied): ")
	}
	b.WriteString(summary)
	if !res.Applied && res.OldHash != "" {
		fmt.Fprintf(&b, "\nbase_hash: %s", res.OldHash)
	}
	if res.Diff != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(res.Diff, "\n"))
	}
	return b.String()
}

// verb picks the applied or preview phrasing for a summary.
func verb(preview bool, applied, would string) string {
	if preview {
		return would
	}
	return applied
}

// previewProperty is the schema fragment shared by every editing tool.
const previewProperty = `"preview": {
		"type": "boolean",
		"description": "When true, show the resulting diff without changing anything."
	}`

// baseHashProperty guards edits against concurrent modification.
const baseHashProperty = `"base_hash": {
		"type": "string",
		"description": "Optional content hash from an earlier preview; the edit fails if the file changed since."
	}`

// CreateFile is the built-in tool that creates a new file.
type CreateFile struct{ engine *edit.Engine }

// NewCreateFile returns the create_file tool backed by engine.
func NewCreateFile(engine *edit.Engine) *CreateFile {
	return &CreateFile{engine: engine}
}

// Definition implements tool.Tool.
func (*CreateFile) Definition() types.Tool {
	return types.Tool{
		Name: "create_file",
		Description: "Create a new text file with the given content. " +
			"Fails if the file already exists; use write_file to overwrite. " +
			"The path is relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path of the file to create."},
				"content": {"type": "string", "description": "Full content of the new file."},
				` + previewProperty + `
			},
			"required": ["path", "content"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*CreateFile) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (c *CreateFile) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	args, err := parseEditArgs("create_file", input)
	if err != nil {
		return "", err
	}
	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}
	res, err := runEdit(ctx, c.engine, edit.Request{
		Op: edit.OpCreate, Path: path, Display: args.Path, Content: args.Content,
	}, args.Preview)
	if err != nil {
		return "", err
	}
	return editContent(res, fmt.Sprintf("%s %s (%d bytes)",
		verb(args.Preview, "created", "would create"), args.Path, res.Bytes)), nil
}

// WriteFile is the built-in tool that sets a file's entire contents.
type WriteFile struct{ engine *edit.Engine }

// NewWriteFile returns the write_file tool backed by engine.
func NewWriteFile(engine *edit.Engine) *WriteFile {
	return &WriteFile{engine: engine}
}

// Definition implements tool.Tool.
func (*WriteFile) Definition() types.Tool {
	return types.Tool{
		Name: "write_file",
		Description: "Write a text file's entire contents, creating it if absent. " +
			"Prefer replace_text or insert_text for partial changes. " +
			"The path is relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path of the file to write."},
				"content": {"type": "string", "description": "Full new content of the file."},
				` + baseHashProperty + `,
				` + previewProperty + `
			},
			"required": ["path", "content"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*WriteFile) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (w *WriteFile) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	args, err := parseEditArgs("write_file", input)
	if err != nil {
		return "", err
	}
	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}
	res, err := runEdit(ctx, w.engine, edit.Request{
		Op: edit.OpWrite, Path: path, Display: args.Path,
		Content: args.Content, BaseHash: args.BaseHash,
	}, args.Preview)
	if err != nil {
		return "", err
	}
	wrote := verb(args.Preview, "wrote", "would write")
	if res.Created {
		wrote = verb(args.Preview, "created", "would create")
	}
	return editContent(res, fmt.Sprintf("%s %s (%d bytes)", wrote, args.Path, res.Bytes)), nil
}

// ReplaceText is the built-in tool that replaces text within a file.
type ReplaceText struct{ engine *edit.Engine }

// NewReplaceText returns the replace_text tool backed by engine.
func NewReplaceText(engine *edit.Engine) *ReplaceText {
	return &ReplaceText{engine: engine}
}

// Definition implements tool.Tool.
func (*ReplaceText) Definition() types.Tool {
	return types.Tool{
		Name: "replace_text",
		Description: "Replace text in a file by exact match. " +
			"The target must occur exactly once unless all is true; " +
			"include surrounding lines to make it unique. " +
			"The path is relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path of the file to edit."},
				"target": {"type": "string", "description": "Exact text to replace."},
				"replacement": {"type": "string", "description": "Text to substitute for the target."},
				"all": {"type": "boolean", "description": "Replace every occurrence instead of requiring a unique match."},
				` + baseHashProperty + `,
				` + previewProperty + `
			},
			"required": ["path", "target", "replacement"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*ReplaceText) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (r *ReplaceText) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	args, err := parseEditArgs("replace_text", input)
	if err != nil {
		return "", err
	}
	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}
	res, err := runEdit(ctx, r.engine, edit.Request{
		Op: edit.OpReplace, Path: path, Display: args.Path,
		Target: args.Target, Replacement: args.Replacement,
		All: args.All, BaseHash: args.BaseHash,
	}, args.Preview)
	if err != nil {
		return "", err
	}
	return editContent(res, fmt.Sprintf("%s %d occurrence(s) in %s",
		verb(args.Preview, "replaced", "would replace"), res.Occurrences, args.Path)), nil
}

// InsertText is the built-in tool that inserts lines into a file.
type InsertText struct{ engine *edit.Engine }

// NewInsertText returns the insert_text tool backed by engine.
func NewInsertText(engine *edit.Engine) *InsertText {
	return &InsertText{engine: engine}
}

// Definition implements tool.Tool.
func (*InsertText) Definition() types.Tool {
	return types.Tool{
		Name: "insert_text",
		Description: "Insert text as new lines at a 1-based line position: " +
			"line N inserts before the current line N, and lineCount+1 appends. " +
			"The path is relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path of the file to edit."},
				"line": {"type": "integer", "description": "1-based line position to insert at."},
				"text": {"type": "string", "description": "Text to insert as new lines."},
				` + baseHashProperty + `,
				` + previewProperty + `
			},
			"required": ["path", "line", "text"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*InsertText) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (i *InsertText) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	args, err := parseEditArgs("insert_text", input)
	if err != nil {
		return "", err
	}
	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}
	res, err := runEdit(ctx, i.engine, edit.Request{
		Op: edit.OpInsert, Path: path, Display: args.Path,
		Line: args.Line, Text: args.Text, BaseHash: args.BaseHash,
	}, args.Preview)
	if err != nil {
		return "", err
	}
	return editContent(res, fmt.Sprintf("%s text at line %d of %s",
		verb(args.Preview, "inserted", "would insert"), args.Line, args.Path)), nil
}

// DeleteText is the built-in tool that removes text from a file.
type DeleteText struct{ engine *edit.Engine }

// NewDeleteText returns the delete_text tool backed by engine.
func NewDeleteText(engine *edit.Engine) *DeleteText {
	return &DeleteText{engine: engine}
}

// Definition implements tool.Tool.
func (*DeleteText) Definition() types.Tool {
	return types.Tool{
		Name: "delete_text",
		Description: "Delete text from a file by exact match. " +
			"The target must occur exactly once unless all is true. " +
			"The path is relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Path of the file to edit."},
				"target": {"type": "string", "description": "Exact text to delete."},
				"all": {"type": "boolean", "description": "Delete every occurrence instead of requiring a unique match."},
				` + baseHashProperty + `,
				` + previewProperty + `
			},
			"required": ["path", "target"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*DeleteText) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (d *DeleteText) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	args, err := parseEditArgs("delete_text", input)
	if err != nil {
		return "", err
	}
	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}
	res, err := runEdit(ctx, d.engine, edit.Request{
		Op: edit.OpDelete, Path: path, Display: args.Path,
		Target: args.Target, All: args.All, BaseHash: args.BaseHash,
	}, args.Preview)
	if err != nil {
		return "", err
	}
	return editContent(res, fmt.Sprintf("%s %d occurrence(s) from %s",
		verb(args.Preview, "deleted", "would delete"), res.Occurrences, args.Path)), nil
}

// RenameFile is the built-in tool that moves a file to a new path.
type RenameFile struct{ engine *edit.Engine }

// NewRenameFile returns the rename_file tool backed by engine.
func NewRenameFile(engine *edit.Engine) *RenameFile {
	return &RenameFile{engine: engine}
}

// Definition implements tool.Tool.
func (*RenameFile) Definition() types.Tool {
	return types.Tool{
		Name: "rename_file",
		Description: "Rename a file to a new path within the working directory. " +
			"Fails if the destination already exists. " +
			"Both paths are relative to the working directory.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "Current path of the file."},
				"new_path": {"type": "string", "description": "New path for the file."},
				` + previewProperty + `
			},
			"required": ["path", "new_path"]
		}`),
	}
}

// Permission implements tool.Tool.
func (*RenameFile) Permission() tool.Permission { return tool.PermissionWrite }

// Execute implements tool.Tool.
func (r *RenameFile) Execute(ctx context.Context, exec tool.ExecContext, input json.RawMessage) (string, error) {
	args, err := parseEditArgs("rename_file", input)
	if err != nil {
		return "", err
	}
	if args.NewPath == "" {
		return "", types.NewError(types.ErrorKindValidation, "rename_file: new_path is required")
	}
	path, err := resolve(exec, args.Path)
	if err != nil {
		return "", err
	}
	newPath, err := resolve(exec, args.NewPath)
	if err != nil {
		return "", err
	}
	res, err := runEdit(ctx, r.engine, edit.Request{
		Op: edit.OpRename, Path: path, Display: args.Path,
		NewPath: newPath, NewDisplay: args.NewPath,
	}, args.Preview)
	if err != nil {
		return "", err
	}
	return editContent(res, fmt.Sprintf("%s %s to %s",
		verb(args.Preview, "renamed", "would rename"), args.Path, args.NewPath)), nil
}
