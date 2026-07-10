package terminal

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompt asks the user for a line of input, echoed as typed. The prompt
// itself is written to Err so stdout stays clean for command output.
func (t *Terminal) Prompt(label string) (string, error) {
	fmt.Fprintf(t.Err, "%s: ", label)

	line, err := t.Reader().ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("terminal: read input: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// PromptSecret asks the user for a secret value. When input is an
// interactive terminal the value is read without echoing; otherwise
// (tests, pipes) it falls back to reading a line. The prompt itself is
// written to Err so stdout stays clean for command output.
func (t *Terminal) PromptSecret(label string) (string, error) {
	fmt.Fprintf(t.Err, "%s: ", label)

	if f, ok := t.In.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		value, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(t.Err)
		if err != nil {
			return "", fmt.Errorf("terminal: read secret: %w", err)
		}
		return string(value), nil
	}

	line, err := t.Reader().ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("terminal: read secret: %w", err)
	}
	return strings.TrimSpace(line), nil
}

// Notify displays an instruction to the user on Err, keeping stdout
// clean for command output.
func (t *Terminal) Notify(message string) {
	fmt.Fprintln(t.Err, message)
}
