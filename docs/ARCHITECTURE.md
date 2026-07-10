# Architecture

Slick Code CLI is organized around two extension points: the
`provider.Provider` interface (`internal/provider/provider.go`) through
which the application talks to AI providers, and the `tool.Tool`
interface (`internal/tool/tool.go`) through which the assistant executes
tools on the user's machine. Everything else exists to support commands
calling into providers and tools through those interfaces.

## Domain model

`pkg/types` is the single source of truth for every concept the
application exchanges with providers: conversations, messages, content
parts, tools, models, capabilities, usage, responses, stream events,
embeddings, image generation, and errors. Provider implementations
translate their own API's request/response formats into these types at
the integration boundary — provider-specific structures never appear
anywhere else in the codebase, so the domain stays stable even when a
provider changes its API.

Two closed unions are modeled as sealed interfaces (an unexported marker
method restricts implementations to `pkg/types`):

- `types.Part` — message content: `TextPart`, `ReasoningPart`,
  `ImagePart`, `FilePart`, `ToolCallPart`, `ToolResultPart`.
- `types.StreamEvent` — streaming increments: `TextEvent`,
  `ReasoningEvent`, `ToolCallEvent`, `DoneEvent`.

Consumers type switch over them; the compiler and the sealed set together
make invalid content unrepresentable.

## Provider contracts

Every provider implements the base `provider.Provider` interface
(`Name`, `Models`). Operations are optional capability interfaces in
`internal/provider/contracts.go`, discovered by type assertion — the
same pattern as `http.Pusher`:

- `Completer` — single-exchange chat
- `Streamer` — incremental responses (`iter.Seq2[types.StreamEvent, error]`)
- `Embedder` — embedding vectors
- `ImageGenerator` — image generation

`provider.Capability[T](registry, name)` performs the lookup and
assertion, returning a typed `unsupported_capability` error when the
provider doesn't implement `T`. Adding a new operation means adding an
interface, not extending a switch. Model-level capabilities (vision,
reasoning, tools, ...) are declared per model via
`types.Model.Capabilities`, because they vary between a provider's
models rather than per provider.

### OpenAI provider

`internal/provider/openai` implements the existing `Provider`,
`Authenticator`, lifecycle/health, `Completer`, and `Streamer` contracts.
Bootstrap constructs it with the shared HTTP client/logger and registers it
beside Anthropic; the runtime, authentication manager, configuration, and REPL
contain no OpenAI-specific branch.

The package targets `/v1/models` and `/v1/chat/completions` and keeps separate
client, API-key flow, translation, streaming, and error files. It translates
shared text, tool definitions, assistant tool calls, tool results, usage,
stop reasons, and streamed tool-call fragments. Vision/file content is
deliberately rejected as unsupported.

OpenAI login reuses `auth.APIKeyFlow`, secure layered credential storage,
session validation, and lifecycle activation. Credentials remain `auth.Secret`
and are revealed only for the Bearer header; CI can use the existing
`SLICKCODE_OPENAI_API_KEY` convention. Requests reuse `transport.DoWithRetry`,
bounded response/SSE decoding, shared cancellation and User-Agent behavior,
and structured status/timing logs. Errors are mapped to shared domain kinds.
`SLICKCODE_OPENAI_BASE_URL` is a validated HTTP(S) override for compatible
gateways and local tests.

### Ollama provider

`internal/provider/ollama` implements the same provider/chat/stream contracts
using Ollama's native `/api/tags`, `/api/show`, and `/api/chat` endpoints. It
reuses `auth.MethodNone`/`auth.NoneFlow`: normal lifecycle activation requires
no login, key, OAuth flow, or secret storage. Activation performs a local
health check before the provider becomes active.

Endpoint resolution prefers `SLICKCODE_OLLAMA_BASE_URL`, then the standard
`OLLAMA_HOST` variable, then `http://127.0.0.1:11434`. Values are validated as
credential-free HTTP(S) URLs. Installed model names are preserved exactly;
there are no aliases or provider-independent rewrites. `/api/show`
capabilities distinguish chat models and advertise tools/thinking only when
the installed model reports them.

Ollama adds the optional provider-independent `ModelValidator` contract.
Before the REPL starts, providers implementing it validate the exact configured
model. Missing Ollama models return an actionable `ollama pull <model>` error;
an empty model still follows the existing setup/model-selection flow. Native
NDJSON streaming produces shared text, reasoning, tool-call, and done events.
Requests reuse shared retries, cancellation, bounded decoding, logging, tools,
editing/context/Git input, and classified domain errors.

## Authentication and provider lifecycle

`internal/auth` is the provider-independent authentication framework.
The division of knowledge:

- `internal/auth` knows how to drive each authentication *kind* — API
  key, browser OAuth, device code, none — via the flow contracts in
  `flow.go` (`APIKeyFlow`, `BrowserFlow`, `DeviceCodeFlow`, `NoneFlow`).
  The choreography for each kind lives once, in `auth.Manager.Login`, so
  the login experience is identical across providers. Adding a new
  authentication strategy means adding a flow contract and its
  choreography here; providers and the runtime are untouched.
- Providers declare what they support through the optional
  `provider.Authenticator` interface (`AuthMethods`, `NewFlow`) and may
  implement `auth.Refresher` for non-interactive session renewal — both
  discovered by type assertion like every other capability.
- `provider.Lifecycle` owns provider runtime state (registered → active
  → deactivated). `Activate` resolves the session through
  `auth.Manager` — refreshing an expired credential when the provider
  implements `auth.Refresher`, failing with a typed authentication
  error otherwise — then runs the provider's optional `Activator`,
  `Deactivator`, and `HealthChecker` hooks.

### Security model

- Secrets use the `auth.Secret` type, whose `fmt` and `slog`
  representations are always `[REDACTED]`; only an explicit `Reveal()`
  call yields the raw value. A secret cannot leak through a log
  statement or formatted error by accident.
- Credentials are stored in the operating system's native vault
  (Keychain, Credential Manager, Secret Service) via
  `internal/auth/keyring`, as JSON payloads inside the vault — never in
  plain-text files, and entirely separate from `config.yaml`. The
  `auth.Store` interface allows alternative backends (e.g. an encrypted
  file store) without changing any public API. `auth.MemoryStore`
  backs tests.
- Authentication state (`auth.Session`) is a separate concept from
  provider runtime state (`Lifecycle`'s active set).

## Tool execution

`internal/tool` is the framework through which the assistant executes
tools on the user's machine. It mirrors the provider design: a `Tool`
interface, an instantiated `Registry` (registration and discovery, no
global state), and a `Manager` that owns the execution pipeline. The
framework knows nothing about any specific tool; built-ins live in
`internal/tool/builtin`.

`Manager.Execute` runs one `types.ToolCall` through four steps —
resolve the tool from the registry, check the permission `Policy`,
honor dry-run requests without executing, and run the tool under a
time budget (`ExecContext.Timeout`, default 30s). It never returns a Go
error: every failure is classified into a `types.Error`
(`validation`, `permission_denied`, `timeout`, ...) and folded into the
`Result`, which converts to the `types.ToolResult` handed back to the
model.

- Tools declare the access level they need (`PermissionRead`,
  `PermissionWrite`, `PermissionExecute`); the `Policy` decides what is
  granted. Bootstrap grants read and write access; the policy remains
  the single authority over what may run — the editing engine has no
  side door around it.
- `ExecContext.WorkDir` is the boundary: built-in tools resolve every
  path against it and refuse paths that escape it.
- The read-only built-ins are `read_file` (size-capped) and
  `list_directory`. The mutating built-ins (`create_file`, `write_file`,
  `replace_text`, `insert_text`, `delete_text`, `rename_file`) are thin
  adapters over the editing engine below. Git integration and the
  assistant-driven tool loop come later.

## File editing engine

`internal/edit` is the single subsystem through which files are
modified. It is provider-independent (it depends only on `pkg/types`
and the standard library) and knows nothing about tools or models;
`internal/tool/builtin` adapts it into the tool framework, so providers
request edits exclusively as tool calls executed by the tool `Manager` —
one execution path, one permission authority.

The `edit.Engine` plans every edit fully in memory before touching
disk: it validates the `edit.Request`, reads the current file state,
detects conflicts, computes the new content, and renders a unified
diff. Only then does `Apply` commit — via an atomic write (temp file in
the same directory, flush, rename) — so a failed or interrupted edit
never leaves a file half-modified. `Preview` runs the same plan and
returns the `edit.Result` (diff, occurrence counts, content hashes)
without writing anything.

Safety properties:

- **Conflict detection** — every result carries SHA-256 content hashes;
  passing a preview's `OldHash` as the next request's `BaseHash` makes
  the apply fail with a typed `conflict` error if the file changed in
  between. Creating over an existing file and renaming onto an existing
  destination are also conflicts.
- **Rollback** — every applied edit yields a `Rollback` token capturing
  the prior bytes and mode; `Engine.Rollback` restores it, refusing if
  the file was modified after the edit so later work is never
  clobbered. Applied edits also land in a bounded in-memory journal
  consumed by `Engine.Undo`.
- **Data preservation** — the encoding (UTF-8, UTF-8 BOM, UTF-16LE/BE)
  is detected on read and restored on write; binary files are refused.
  Replace and delete match exact text so untouched bytes — including
  line endings — pass through unchanged; insert adopts the file's
  dominant line ending.
- **Bounded and cancelable** — file and content sizes are capped,
  context cancellation is honored between planning and commit, and the
  tool `Manager`'s time budget applies to every tool-initiated edit.

Every applied edit, preview, and rollback is logged with structured
fields (operation, path, hashes, byte counts, duration).

## Project context engine

`internal/workspace` and `internal/context` form the provider-independent
project context engine. Bootstrap discovers the workspace once from the
process working directory. A containing Git repository is the root; outside
Git, the nearest recognized project marker is used. Discovery also records
nested workspaces without invoking Git or another external program.

The collector walks from that root, applying root and nested `.gitignore`
files in declaration order. It skips version-control metadata,
dependency/build directories, binary files, oversized files, and bounded
total input. Every collected text file carries a root-relative slash path,
language, size, mode, modification time, SHA-256 content hash, and content.
Unchanged files are reused when their size, mode, and nanosecond modification
time match the previous scan.

`context.Service` owns the collected file cache and current bounded snapshot:

- `HeuristicEstimator` provides deterministic, provider-neutral token
  estimates without adding a tokenizer dependency.
- `Budget` is the single token-ceiling authority.
- `Builder` writes a stable project manifest and then includes high-value
  files (project metadata, source, documentation, then other text) while they
  fit the budget.
- `Service.Refresh` incrementally updates changed and removed files, rebuilds
  the snapshot, and atomically persists a mode-`0600` cache under the
  platform cache directory. Cache entries are scoped by project root.

The REPL refreshes before a turn, prepends the snapshot as a system message,
and exposes `/context [refresh]` for status and manual refresh. The
`project_context` read-permission tool provides the same coverage and refresh
information through the existing tool manager. Discovery, collection, and
refresh emit structured log events through the injected `*slog.Logger`.

## Git integration

`internal/git` is the provider-independent repository boundary. It uses the
installed Git CLI rather than reimplementing repository formats, but every
invocation is bounded and non-interactive: commands inherit cancellation and
tool deadlines, capture at most 4 MiB per output stream, disable terminal
prompts and pagers, force stable `C` output, and isolate repository hooks.

`git.Discoverer` resolves the repository root and metadata directory from the
runtime working directory. `git.Manager` is constructed once for that
repository and owns all repository operations: metadata/current branch,
porcelain status and changed files, working/staged diffs, commit creation,
working-tree restore, existing-branch checkout, and branch creation. Path
arguments are normalized beneath the repository root and passed after `--`.
Git failures are translated to shared typed validation, conflict, permission,
cancellation, timeout, and internal errors. Operations emit structured start,
completion, failure, commit, restore, checkout, and branch events.

Providers do not import or call `internal/git`. Bootstrap registers six
built-in tools through the existing registry: `git_status`, `git_diff`,
`git_commit`, `git_restore`, `git_checkout`, and `git_branch`. Read tools use
`tool.PermissionRead`; repository mutations use `tool.PermissionWrite`, so
permission denial, dry-run, timeout, cancellation, and result correlation all
remain centralized in `tool.Manager`. Git state is also composed into the
Project Context Engine snapshot; there is no second execution path.

## Error model

`types.Error` classifies every failure into an `ErrorKind`
(authentication, rate limit, invalid config, unsupported capability,
network, provider, validation, permission denied, timeout, conflict,
internal). Providers map their API errors
into these kinds at the boundary; `types.KindOf(err)` classifies any
error for exit codes, retries, or user-facing hints. `types.Error`
supports `errors.Is`/`errors.As` traversal via `Unwrap`.

## Package responsibilities

| Package                                                   | Responsibility                                             |
| ---------------------------------------------------------- | ----------------------------------------------------------- |
| `cmd/slickcode`                                             | Entrypoint: runs bootstrap, hands off to the runtime         |
| `internal/runtime`                                          | Process lifecycle: builds the command tree, graceful shutdown |
| `internal/command`                                          | Cobra command tree; thin, delegates to `internal/core`      |
| `internal/core`                                             | Composition root: `App`, `Params`, and `Bootstrap`           |
| `internal/config`                                           | Loads and validates configuration from file and environment  |
| `internal/logging`                                          | Structured logger construction (`log/slog`)                  |
| `internal/provider`                                         | `Provider` interface and `Registry`                           |
| `internal/provider/anthropic`                               | Anthropic Messages API integration                           |
| `internal/provider/openai`                                  | OpenAI models, Chat Completions, tools, and SSE               |
| `internal/provider/ollama`                                  | Ollama local discovery, chat, tools, and NDJSON streaming     |
| `internal/auth`                                             | Authentication framework: methods, flows, sessions, manager |
| `internal/auth/keyring`                                     | OS-native secure credential storage                          |
| `internal/repl`                                             | Interactive assistant: Bubble Tea UI, streaming, slash commands |
| `internal/setup`                                            | First-run setup wizard                                       |
| `internal/tool`                                             | Tool execution framework: registry, policy, manager          |
| `internal/tool/builtin`                                     | Built-in tools: read-only and editing tool adapters          |
| `internal/edit`                                             | File editing engine: plan, diff, atomic apply, rollback      |
| `internal/workspace`                                        | Project discovery, ignores, walking, file metadata           |
| `internal/context`                                          | Context building, budgeting, cache, incremental refresh      |
| `internal/git`                                              | Repository discovery, metadata, diffs, commits, branches     |
| `internal/session`                                          | In-memory conversation state                                 |
| `internal/terminal`                                         | IO surface (`Terminal`) and Lip Gloss styles                  |
| `internal/transport`                                        | Shared HTTP client                                             |
| `internal/storage`                                          | On-disk config directory resolution (`Paths`)                 |
| `pkg/types`                                                 | Stable, public data types                                     |
| `pkg/version`                                               | Build metadata                                                |

## Dependency direction

Bottom-up, each layer only depends on the ones below it:

```
cmd/slickcode
  -> internal/runtime
       -> internal/command
            -> internal/repl, internal/setup
            -> internal/core
                 -> internal/config, internal/logging, internal/storage,
                    internal/terminal, internal/auth, internal/provider,
                    internal/tool, internal/edit, internal/workspace,
                    internal/context, internal/git, internal/transport
                      -> pkg/types
```

`internal/session`, `internal/transport`, and `internal/edit` depend
only on `pkg/types` and the standard library. Provider subpackages
(`internal/provider/<name>`) and tool subpackages
(`internal/tool/builtin`, which adapts `internal/edit` into tools) are
imported by nothing except their registration calls in
`internal/core.Bootstrap`.

## Configuration, logging, and the application lifecycle

`core.Bootstrap()` is the single place that assembles a running
application:

1. `storage.Discover()` resolves the config directory.
2. `config.Load(path)` reads defaults, the config file, and environment
   variables, in that order of increasing precedence.
3. `Config.Validate()` rejects unrecognized `log_level` or
   `default_provider` values before anything else runs.
4. `logging.New(level)` builds the `*slog.Logger` used for diagnostic
   output on stderr, separate from the `Terminal` used for command
   output.
5. Workspace and Git repository discovery run from the process working
   directory. The repository manager is injected into project context and Git
   tools when a repository exists.
6. The cached project context service is constructed from the workspace,
   optional repository manager, and platform cache path.
7. The provider registry, tool registry (including `project_context` and six
   Git tools), auth manager, provider lifecycle, and `terminal.New()` complete
   the `App`.

`runtime.Runtime.Run(ctx)` then builds the command tree and executes it
under a context that's canceled on SIGINT/SIGTERM
(`signal.NotifyContext`), so future long-running commands can shut down
cleanly instead of being killed. Errors returned from `Run` propagate back
to `cmd/slickcode`, which is the only place that turns them into an exit
code.

## Adding a provider

See [CONTRIBUTING.md](../CONTRIBUTING.md#adding-a-provider).

## Full documentation

User-facing documentation, guides, and release notes are published on the
[Slick Code website](https://github.com/melonyzu/slick-code-website), a
separate repository. This file covers only the CLI's internal architecture
for contributors.
