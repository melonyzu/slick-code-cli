package repl

import (
	"slices"
	"strings"
	"testing"
)

func TestCommandTableIsWellFormed(t *testing.T) {
	cmds := commands()

	required := []string{
		"help", "exit", "clear", "history", "models", "model",
		"provider", "status", "config", "context", "version", "doctor", "logout", "reset",
	}

	seen := map[string]bool{}
	for _, c := range cmds {
		if c.name == "" || c.summary == "" || c.run == nil {
			t.Errorf("command %+v is missing fields", c)
		}
		for _, name := range append([]string{c.name}, c.aliases...) {
			if seen[name] {
				t.Errorf("duplicate command name/alias %q", name)
			}
			seen[name] = true
		}
	}

	for _, name := range required {
		if !seen[name] {
			t.Errorf("required command /%s is missing", name)
		}
	}
	if !seen["quit"] {
		t.Error("/quit alias is missing")
	}

	if !slices.IsSortedFunc(cmds, func(a, b slashCommand) int {
		return strings.Compare(a.name, b.name)
	}) {
		t.Error("command table must be sorted by name for /help output")
	}
}
