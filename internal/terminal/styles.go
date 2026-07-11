// Package terminal defines the Lip Gloss styles shared across Slick Code's
// command-line output and interactive Bubble Tea views.
package terminal

import "github.com/charmbracelet/lipgloss"

// Shared styles for CLI output.
var (
	// Title styles headings and primary labels.
	Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("229"))

	// Accent styles highlighted text.
	Accent = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

	// Prompt styles the input prompt.
	Prompt = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

	// Divider styles separators.
	Divider = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))

	// Success styles confirmation messages.
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	// Error styles error messages.
	Error = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// Muted styles secondary text.
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)
