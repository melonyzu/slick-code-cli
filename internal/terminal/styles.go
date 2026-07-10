// Package terminal defines the Lip Gloss styles shared across Slick Code's
// command-line output and interactive Bubble Tea views, keeping presentation
// consistent in one place.
package terminal

import "github.com/charmbracelet/lipgloss"

// Shared styles for CLI output.
var (
	// Title styles headings, such as command banners.
	Title = lipgloss.NewStyle().Bold(true)

	// Success styles confirmation messages.
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	// Error styles error messages.
	Error = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	// Muted styles secondary, low-emphasis text.
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)
