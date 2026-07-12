# Slick Code CLI

Slick Code CLI is a terminal-based coding assistant for working with multiple AI providers through one command-line interface.

It is built around one simple idea: the CLI should stay consistent even when the provider changes. You should be able to switch providers, models, and authentication methods without learning a different tool every time.

Slick Code combines:
- a Bubble Tea terminal interface,
- provider-specific model and chat support,
- secure credential storage,
- workspace-aware project context,
- built-in tools,
- safe file editing,
- and Git integration.

The result is a single developer workflow that can stay in the terminal from the first prompt to the final edit.

## Table of contents

- [Why Slick Code exists](#why-slick-code-exists)
- [Design goals](#design-goals)
- [What it does](#what-it-does)
- [Provider support](#provider-support)
- [How the runtime is put together](#how-the-runtime-is-put-together)
- [Workspace and context flow](#workspace-and-context-flow)
- [Tool execution](#tool-execution)
- [Safe editing](#safe-editing)
- [Authentication](#authentication)
- [Configuration](#configuration)
- [Installation](#installation)
- [Usage](#usage)
- [Commands](#commands)
- [Project layout](#project-layout)
- [Development](#development)
- [Contributing](#contributing)
- [Security](#security)
- [Roadmap](#roadmap)
- [License](#license)

## Why Slick Code exists

Most AI coding tools are built around one provider or one interface. That works until you want to compare models, switch providers, or keep the same terminal workflow while changing the backend.

Slick Code takes the opposite approach.

The terminal experience stays the same:
- the same assistant window,
- the same slash commands,
- the same context pipeline,
- the same tool framework,
- and the same editing flow.

Only the provider changes.

That makes the CLI easier to maintain and easier to extend. It also keeps the codebase provider-independent: provider-specific behavior stays inside provider packages, while the runtime, tool manager, and UI keep talking in the shared domain model.

## Design goals

Slick Code is designed to be:

| Goal | What that means in practice |
| --- | --- |
| Provider-agnostic | Anthropic, OpenAI, Ollama, OpenRouter, and future providers all fit the same runtime |
| Terminal-first | The entire experience works in the terminal, not in a browser |
| Safe by default | Credentials are kept in the OS keyring, edits are validated, and workspace boundaries are enforced |
| Context-aware | The assistant sees a token-budgeted view of the project instead of the whole repository |
| Extensible | Providers and tools plug into shared interfaces instead of ad hoc code paths |
| Maintainable | Packages are small, responsibilities are separated, and public APIs stay narrow |
| Practical | The CLI is meant to be used, not just admired |

## What it does

Slick Code can:

- start an interactive assistant in the terminal,
- discover the current project automatically,
- collect only relevant files into a bounded project context,
- stream responses from supported providers,
- run built-in tools,
- edit files safely,
- show Git state and perform common Git operations,
- and keep provider-specific details out of the rest of the runtime.

A typical session looks like this:

1. Start the CLI.
2. Choose a provider.
3. Load or create configuration.
4. Authenticate if needed.
5. Discover the workspace.
6. Build project context.
7. Ask a question.
8. Let the assistant read, write, or inspect files through tools.
9. Continue the conversation in the same terminal session.

## Provider support

Slick Code currently supports these providers:

| Provider | Model discovery | Authentication | Notes |
| --- | --- | --- | --- |
| Anthropic | Yes | API key | Cloud provider support |
| OpenAI | Yes | API key | Cloud provider support |
| Ollama | Yes | None | Local model support |
| OpenRouter | Yes | API key | Multi-model routing |

Providers use the same shared contracts, so the runtime can talk to them without branching into provider-specific code paths.

## How the runtime is put together

The runtime is split into a few clear layers.

### Core

`internal/core` wires the app together:
- configuration,
- logging,
- providers,
- authentication,
- workspace discovery,
- context building,
- tool registration,
- and the terminal UI.

This is the composition root of the application.

### Runtime

`internal/runtime` owns lifecycle concerns:
- startup,
- shutdown,
- signal handling,
- and graceful exit.

### REPL

`internal/repl` is the interactive assistant:
- Bubble Tea model,
- transcript rendering,
- streaming updates,
- slash commands,
- history,
- and user input handling.

### Provider layer

`internal/provider` defines provider contracts and registry behavior. Provider packages implement those contracts without knowing about the rest of the UI.

### Tool layer

`internal/tool` defines the tool framework:
- registration,
- discovery,
- permissions,
- dry-run support,
- and execution budgets.

Built-in tools live under `internal/tool/builtin`.

### Editing engine

`internal/edit` is responsible for file changes:
- preview,
- apply,
- rollback,
- conflict detection,
- and atomic writes.

### Context layer

`internal/workspace` and `internal/context` discover the workspace, collect files, apply ignore rules, estimate token usage, and assemble the final context that goes into the assistant prompt.

## Workspace and context flow

The project context pipeline is one of the main pieces that makes Slick Code useful.

At startup the CLI:
1. discovers the workspace root,
2. detects whether the workspace is a Git repository,
3. walks the project tree,
4. applies `.gitignore` rules,
5. skips binary and irrelevant files,
6. estimates token cost,
7. and builds a bounded context snapshot.

That snapshot is then attached to assistant requests as system context.

The goal is not to dump the entire repository into the prompt. The goal is to build a focused view of the project that stays small enough to fit the budget while still being useful.

This matters because:
- large repositories do not fit in a model context window,
- not every file is relevant,
- and rebuilds should avoid rescanning unchanged files when possible.

The context engine is designed around those constraints.

## Tool execution

Slick Code exposes tools through a shared manager instead of letting providers touch the filesystem directly.

That keeps tool execution in one place:
- permissions are checked centrally,
- dry-run behavior is consistent,
- timeouts are enforced once,
- and the assistant sees tool output in the same structured format every time.

Built-in tools currently cover:
- reading files,
- listing directories,
- project context inspection,
- file editing,
- and Git operations.

This is important for two reasons:

1. The provider does not become a special case.
2. The runtime keeps the final authority over what is allowed to run.

## Safe editing

Editing is handled by a dedicated engine in `internal/edit`.

The engine plans an edit in memory first, then applies it atomically only after validation succeeds.

That means:
- requests are validated before touching the filesystem,
- stale edits can be rejected with conflict errors,
- the original file state can be restored through rollback tokens,
- encodings and line endings are preserved where possible,
- and previews can show the change before anything is written.

This is a better fit for AI-assisted editing than raw file writes because the assistant can preview, revise, and retry without leaving half-finished changes behind.

## Authentication

Authentication is kept separate from configuration.

Credentials are not stored in plain-text config files. They are handled by the auth subsystem and persisted through secure credential storage.

The auth layer supports:
- API key flows,
- local/no-auth flows,
- session handling,
- credential lookup,
- validation,
- and refresh where applicable.

That keeps secrets out of the repository and out of local config files.

## Configuration

Slick Code loads configuration from the platform-specific config directory and supports environment variable overrides.

The important values are:
- provider,
- model,
- and log level.

Environment variables use the `SLICKCODE_` prefix.

Example:

```yaml
provider: openai
model: gpt-5.5
log_level: warn
```

Configuration precedence works like this:

1. built-in defaults,
2. config file,
3. environment variables.

That means you can keep a sane default in code, override it locally, and still control it from the environment when needed.

## Installation

### Build from source

```sh
git clone https://github.com/melonyzu/slick-code-cli.git
cd slick-code-cli
make build
```

### Run without building a binary

```sh
go run ./cmd/slickcode
```

### Requirements

- Go 1.24+ (or the Go version declared in `go.mod`)
- A supported shell or terminal
- Provider credentials where applicable

## Usage

Start the CLI:

```sh
slickcode
```

Useful commands:

```sh
slickcode auth login
slickcode auth logout
slickcode auth status
slickcode version
```

Inside the assistant, use:

```text
/help
```

to list the available slash commands.

A normal first run will:
- ask you to choose a provider,
- discover available models,
- create or load the local config,
- authenticate if needed,
- and enter the interactive terminal session.

## Commands

The exact set of slash commands may grow over time, but the current CLI includes commands for:

| Command | Purpose |
| --- | --- |
| `/help` | Show available commands |
| `/model` | Change the active model |
| `/models` | List available models |
| `/provider` | Switch provider |
| `/status` | Show session state |
| `/config` | Show current config |
| `/history` | Show chat history |
| `/reset` | Reset the conversation |
| `/clear` | Clear the terminal |
| `/doctor` | Check setup and environment |
| `/logout` | Remove provider credentials |
| `/version` | Show version information |
| `/exit` | Exit the CLI |

## Project layout

| Path | Purpose |
| --- | --- |
| `cmd/slickcode` | CLI entrypoint |
| `internal/core` | Application bootstrap and dependency wiring |
| `internal/runtime` | Process lifecycle |
| `internal/config` | Config loading and validation |
| `internal/logging` | Structured logging |
| `internal/provider` | Provider contracts and registry |
| `internal/provider/anthropic` | Anthropic provider |
| `internal/provider/openai` | OpenAI provider |
| `internal/provider/ollama` | Ollama provider |
| `internal/provider/openrouter` | OpenRouter provider |
| `internal/repl` | Interactive terminal UI |
| `internal/setup` | First-run setup flow |
| `internal/tool` | Tool framework |
| `internal/tool/builtin` | Built-in tools |
| `internal/edit` | Safe file editing engine |
| `internal/context` | Token-budgeted project context |
| `internal/workspace` | Workspace discovery and collection |
| `internal/git` | Git integration |
| `internal/auth` | Authentication and credential storage |
| `internal/terminal` | Shared terminal styling |
| `internal/storage` | Filesystem paths |
| `internal/transport` | Shared HTTP transport and retry logic |
| `pkg/types` | Shared domain types |
| `pkg/version` | Build metadata |

## Development

The repository is structured to keep responsibilities separate and make testing easier.

Common checks:

```sh
gofmt -w .
go vet ./...
go build ./...
go test ./...
```

If the environment supports race tests, run those too.

### Working on providers

When adding a provider:
- implement the shared provider contracts,
- keep provider-specific types inside the provider package,
- translate provider errors into shared error kinds,
- and avoid adding special cases to the runtime.

### Working on tools

When adding a tool:
- register it through the tool registry,
- enforce permissions through the tool manager,
- keep it confined to the workspace,
- and return structured errors.

### Working on editing

When adding or changing edit behavior:
- plan before writing,
- preserve existing content where possible,
- and make rollback possible whenever practical.

## Contributing

Contributions are welcome.

Before opening a pull request, read:
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `SECURITY.md`

Please keep changes focused and keep the architecture consistent with the rest of the project.

## Security

Security-related issues should be reported privately.

Use the repository’s security policy if you find:
- credential leaks,
- workspace escapes,
- permission bypasses,
- tool execution issues,
- or other security-sensitive bugs.

## Roadmap

Slick Code is still evolving.

Likely next steps include:
- better terminal polish,
- improved provider switching,
- more tool coverage,
- stronger context controls,
- better onboarding,
- and more release automation.

## FAQ

### Why build this in Go?

Go is a good fit for terminal tools, background services, provider integrations, and safe concurrent code. It also keeps the runtime simple enough to ship as a single binary.

### Why not just use one provider directly?

Because the provider is not the product. The workflow is.

### Why the focus on project context?

Because an assistant is only useful if it sees the right part of the codebase instead of a random pile of files.

### Why separate tools from providers?

So the runtime can keep control over permissions, timeouts, and edit safety instead of letting each provider invent its own path.

## License

MIT
