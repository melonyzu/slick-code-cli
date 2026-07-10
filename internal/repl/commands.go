package repl

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/melonyzu/slick-code-cli/internal/config"
	"github.com/melonyzu/slick-code-cli/internal/session"
	"github.com/melonyzu/slick-code-cli/pkg/version"
)

// infoMsg is the result of a slash command that finished
// asynchronously.
type infoMsg struct {
	text  string
	isErr bool
}

// slashCommand is one interactive /command. New commands are added by
// appending to the table in commands(); nothing else in the UI loop
// changes.
type slashCommand struct {
	name    string
	aliases []string
	args    string
	summary string
	run     func(m *model, args string) tea.Cmd
}

// commands returns the slash command table, sorted by name.
func commands() []slashCommand {
	cmds := []slashCommand{
		{name: "help", summary: "Show this list of commands",
			run: (*model).cmdHelp},
		{name: "exit", aliases: []string{"quit"}, summary: "Leave Slick Code",
			run: (*model).cmdExit},
		{name: "clear", summary: "Clear the screen (conversation continues)",
			run: (*model).cmdClear},
		{name: "reset", summary: "Start a fresh conversation",
			run: (*model).cmdReset},
		{name: "history", summary: "List the conversation so far",
			run: (*model).cmdHistory},
		{name: "models", summary: "List the provider's available models",
			run: (*model).cmdModels},
		{name: "model", args: "[model-id]", summary: "Show or change the model",
			run: (*model).cmdModel},
		{name: "provider", summary: "Show the active provider",
			run: (*model).cmdProvider},
		{name: "status", summary: "Show session and provider health",
			run: (*model).cmdStatus},
		{name: "config", summary: "Show the configuration",
			run: (*model).cmdConfig},
		{name: "context", args: "[refresh]", summary: "Show or refresh project context",
			run: (*model).cmdContext},
		{name: "version", summary: "Show the Slick Code version",
			run: (*model).cmdVersion},
		{name: "doctor", summary: "Diagnose common problems",
			run: (*model).cmdDoctor},
		{name: "logout", summary: "Remove the stored credential",
			run: (*model).cmdLogout},
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].name < cmds[j].name })
	return cmds
}

// dispatch runs the slash command in input, which starts with "/".
func (m *model) dispatch(input string) tea.Cmd {
	name, args, _ := strings.Cut(strings.TrimPrefix(input, "/"), " ")
	name = strings.ToLower(name)
	args = strings.TrimSpace(args)

	for i := range m.commands {
		c := &m.commands[i]
		if c.name == name || slices.Contains(c.aliases, name) {
			return c.run(m, args)
		}
	}

	m.appendError(fmt.Sprintf("unknown command /%s — try /help", name))
	return nil
}

func (m *model) cmdHelp(string) tea.Cmd {
	var b strings.Builder
	b.WriteString("Commands:\n")
	for _, c := range m.commands {
		name := "/" + c.name
		if c.args != "" {
			name += " " + c.args
		}
		if len(c.aliases) > 0 {
			name += " (/" + strings.Join(c.aliases, ", /") + ")"
		}
		fmt.Fprintf(&b, "  %-24s %s\n", name, c.summary)
	}
	b.WriteString("\nEnter sends · ctrl+j inserts a newline · ctrl+p/ctrl+n recall input history")
	m.appendInfo(b.String())
	return nil
}

func (m *model) cmdExit(string) tea.Cmd {
	m.quitting = true
	return tea.Quit
}

func (m *model) cmdClear(string) tea.Cmd {
	m.entries = nil
	m.refreshViewport()
	return nil
}

func (m *model) cmdReset(string) tea.Cmd {
	m.sess = session.New()
	m.appendInfo("Conversation reset — the assistant starts fresh.")
	return nil
}

func (m *model) cmdHistory(string) tea.Cmd {
	msgs := m.sess.Messages()
	if len(msgs) == 0 {
		m.appendInfo("No conversation yet.")
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Conversation (%d messages):\n", len(msgs))
	for i, msg := range msgs {
		text := msg.Text()
		if r := []rune(text); len(r) > 80 {
			text = string(r[:77]) + "..."
		}
		fmt.Fprintf(&b, "  %2d. %-9s %s\n", i+1, msg.Role, strings.ReplaceAll(text, "\n", " "))
	}
	m.appendInfo(strings.TrimRight(b.String(), "\n"))
	return nil
}

func (m *model) cmdModels(string) tea.Cmd {
	return m.async(func(ctx context.Context) infoMsg {
		models, err := m.prov.Models(ctx)
		if err != nil {
			return infoMsg{text: err.Error(), isErr: true}
		}

		var b strings.Builder
		b.WriteString("Available models:\n")
		for _, mod := range models {
			marker := "  "
			if mod.ID == m.modelID {
				marker = "* "
			}
			fmt.Fprintf(&b, "  %s%-32s %s\n", marker, mod.ID, mod.Name)
		}
		b.WriteString("\nSwitch with /model <model-id>")
		return infoMsg{text: strings.TrimRight(b.String(), "\n")}
	})
}

func (m *model) cmdModel(args string) tea.Cmd {
	if args == "" {
		m.appendInfo(fmt.Sprintf("Current model: %s — change with /model <model-id>", m.modelID))
		return nil
	}

	return m.async(func(ctx context.Context) infoMsg {
		models, err := m.prov.Models(ctx)
		if err != nil {
			return infoMsg{text: err.Error(), isErr: true}
		}
		for _, mod := range models {
			if mod.ID == args {
				return m.switchModel(args)
			}
		}
		return infoMsg{text: fmt.Sprintf("unknown model %q — see /models", args), isErr: true}
	})
}

// switchModel applies and persists a validated model change.
func (m *model) switchModel(id string) infoMsg {
	m.modelID = id
	m.app.Config.Model = id
	path := m.app.Storage.ConfigFile(config.FileName)
	if err := config.Save(path, m.app.Config); err != nil {
		return infoMsg{text: "model switched for this session, but saving failed: " + err.Error(), isErr: true}
	}
	return infoMsg{text: "Model switched to " + id}
}

func (m *model) cmdProvider(string) tea.Cmd {
	registered := make([]string, 0)
	for _, p := range m.app.Providers.List() {
		registered = append(registered, p.String())
	}
	m.appendInfo(fmt.Sprintf("Active provider: %s\nRegistered: %s",
		m.provName, strings.Join(registered, ", ")))
	return nil
}

func (m *model) cmdStatus(string) tea.Cmd {
	return m.async(func(ctx context.Context) infoMsg {
		var b strings.Builder
		fmt.Fprintf(&b, "Provider: %s\nModel:    %s\n", m.provName, m.modelID)

		sess, err := m.app.Auth.Session(ctx, m.provName)
		switch {
		case err != nil:
			fmt.Fprintf(&b, "Session:  %v\n", err)
		case !sess.Valid(time.Now()):
			fmt.Fprintf(&b, "Session:  expired (%s)\n", sess.Method())
		default:
			fmt.Fprintf(&b, "Session:  authenticated (%s)\n", sess.Method())
		}

		if err := m.app.Lifecycle.CheckHealth(ctx, m.provName); err != nil {
			fmt.Fprintf(&b, "Health:   %v", err)
		} else {
			fmt.Fprint(&b, "Health:   ok")
		}
		return infoMsg{text: b.String()}
	})
}

func (m *model) cmdConfig(string) tea.Cmd {
	path := m.app.Storage.ConfigFile(config.FileName)
	m.appendInfo(fmt.Sprintf("Config file: %s\nprovider:  %s\nmodel:     %s\nlog_level: %s",
		path, m.app.Config.Provider, m.app.Config.Model, m.app.Config.LogLevel))
	return nil
}

func (m *model) cmdContext(args string) tea.Cmd {
	if args != "" && args != "refresh" {
		m.appendError("usage: /context [refresh]")
		return nil
	}
	return m.async(func(ctx context.Context) infoMsg {
		changed, removed := 0, 0
		if args == "refresh" {
			result, err := m.app.Context.Refresh(ctx)
			if err != nil {
				return infoMsg{text: err.Error(), isErr: true}
			}
			changed, removed = len(result.Changed), len(result.Removed)
		}
		snapshot := m.app.Context.Snapshot()
		text := fmt.Sprintf("Project context:\nRoot:      %s\nFiles:     %d/%d included\nTokens:    %d/%d estimated\nTruncated: %t",
			snapshot.Root, snapshot.IncludedFiles, snapshot.TotalFiles,
			snapshot.Estimated, snapshot.Budget, snapshot.Truncated)
		if args == "refresh" {
			text += fmt.Sprintf("\nRefresh:   %d changed, %d removed", changed, removed)
		}
		return infoMsg{text: text}
	})
}

func (m *model) cmdVersion(string) tea.Cmd {
	m.appendInfo("slickcode " + version.String())
	return nil
}

func (m *model) cmdDoctor(string) tea.Cmd {
	return m.async(func(ctx context.Context) infoMsg {
		var b strings.Builder
		b.WriteString("Diagnostics:\n")

		check := func(label string, err error) {
			if err != nil {
				fmt.Fprintf(&b, "  ✗ %-16s %v\n", label, err)
			} else {
				fmt.Fprintf(&b, "  ✓ %-16s ok\n", label)
			}
		}

		path := m.app.Storage.ConfigFile(config.FileName)
		var cfgErr error
		if !config.Exists(path) {
			cfgErr = errors.New("missing (first-run setup will recreate it)")
		}
		check("config", cfgErr)

		_, sessErr := m.app.Auth.Session(ctx, m.provName)
		check("credential", sessErr)

		check("provider", m.app.Lifecycle.CheckHealth(ctx, m.provName))

		return infoMsg{text: strings.TrimRight(b.String(), "\n")}
	})
}

func (m *model) cmdLogout(string) tea.Cmd {
	if err := m.app.Auth.Logout(m.ctx, m.provName); err != nil {
		m.appendError(err.Error())
		return nil
	}
	m.appendInfo("Logged out. The current session keeps working; you'll authenticate again next start.")
	return nil
}

// async runs fn off the UI loop and delivers its result as an infoMsg.
func (m *model) async(fn func(context.Context) infoMsg) tea.Cmd {
	parent := m.ctx
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		return fn(ctx)
	}
}
