# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Production Ollama provider (`internal/provider/ollama`) registered through
  the existing registry/lifecycle: local/no-auth activation, configurable and
  standard endpoint discovery, health checks, installed model discovery,
  exact model validation with `ollama pull` guidance, `/api/show` capability
  detection, native chat, NDJSON streaming, reasoning/tool calls, retries,
  bounded decoding, structured logging, and shared error mapping.
- Provider-independent `ModelValidator` startup contract so local configured
  models are verified before the REPL without adding Ollama branches to the
  runtime. Local-only tests cover registration, endpoint priority, no-auth
  sessions, health, models, capabilities, exact names, missing-model errors,
  chat, tools, streaming, retries, and malformed responses.

- Production OpenAI provider (`internal/provider/openai`) registered through
  the existing registry/lifecycle: secure API-key activation, session and
  configuration validation, model discovery/filtering, Chat Completions, SSE
  streaming, capability detection, function tool definitions/calls/results,
  health checks, retries, bounded decoding, structured logging, and shared
  error mapping.
- Local-only OpenAI tests covering registration, authentication, session/base
  URL validation, model capabilities, non-streaming and streamed tool calls,
  tool history, retries, malformed streams, and HTTP error classification.
  Tests never contact OpenAI or require a real API key.

- Provider-independent Git integration (`internal/git`): repository discovery,
  repository/HEAD/branch metadata, porcelain status and changed files,
  working and staged diffs, commit creation, tracked-file restore, existing
  branch checkout, and branch creation.
- Six built-in Git tools registered through the existing Tool Registry:
  `git_status`, `git_diff`, `git_commit`, `git_restore`, `git_checkout`, and
  `git_branch`. Read/write permissions, dry-run, timeout, cancellation, and
  structured results remain enforced by the shared Tool Manager.
- Structured Git errors/logging, non-interactive bounded command execution,
  repository hook isolation, root-confined path handling, Git metadata in
  project context, and behavioral coverage for discovery, status, diffs,
  commits, restore, branches, permissions, dry-run, cancellation, timeout,
  hooks, and error handling.

- Project context engine (`internal/workspace`, `internal/context`): Git and
  project-root discovery, nested workspace detection, nested `.gitignore`
  parsing and matching, bounded directory walking, text/binary filtering,
  language detection, SHA-256 file metadata, deterministic token estimation,
  budgeted context assembly, a mode-`0600` atomic cache, and incremental
  refresh that reuses unchanged files.
- Runtime and interactive project-context integration: assistant turns receive
  the current bounded project snapshot as a system message; `/context
  [refresh]` reports coverage and refreshes changes; the read-permission
  `project_context` tool exposes status through the existing tool manager.
- Structured workspace discovery, collection, and context refresh logs, plus
  unit coverage for discovery, ignore matching, collection, language
  detection, budgeting, cache persistence, and incremental refresh.

- File editing engine (`internal/edit`): the single subsystem through
  which files are modified. Every edit is planned fully in memory —
  request validation, SHA-256 conflict detection (`BaseHash` guards
  against concurrent modification), encoding preservation (UTF-8,
  UTF-8 BOM, UTF-16LE/BE; binary files refused), line-ending
  preservation, and unified diff rendering — then committed with an
  atomic temp-file-and-rename write. Previews report the diff without
  writing; applied edits yield rollback tokens that refuse to clobber
  later modifications, and land in a bounded journal consumed by
  `Engine.Undo`. Edits, previews, and rollbacks are logged with
  structured fields.
- Editing tools (`internal/tool/builtin`): `create_file`, `write_file`,
  `replace_text`, `insert_text`, `delete_text`, and `rename_file`, thin
  adapters over the editing engine registered in the existing tool
  framework — the tool `Manager` and its permission `Policy` remain the
  single execution path and write authority, and every tool accepts
  `preview` and (where relevant) `base_hash` arguments.
- New error kind `conflict` in `pkg/types` for operations refused
  because the state they depend on changed underneath them.

- Tool execution framework (`internal/tool`): the `Tool` interface,
  a `Registry` for registration and discovery, a permission `Policy`
  checked before every call, and a `Manager` that executes tool calls
  with dry-run support, a per-call time budget, and classified
  `types.Error` failures folded into results the model can consume.
- First built-in tools (`internal/tool/builtin`): `read_file` and
  `list_directory`, both read-only and confined to the working
  directory — paths that escape it are refused.
- New error kinds `permission_denied` and `timeout` in `pkg/types`.

- Interactive assistant (`internal/repl`): a Bubble Tea terminal UI
  with streaming responses, markdown rendering, input history recall,
  and slash commands (`/help`, `/model`, `/models`, `/provider`,
  `/status`, `/config`, `/history`, `/reset`, `/clear`, `/doctor`,
  `/logout`, `/version`, `/exit`). Running `slickcode` with no
  subcommand starts it.
- First-run setup wizard (`internal/setup`): choose a provider,
  authenticate, pick a default model, and save the configuration.
- Anthropic provider (`internal/provider/anthropic`): completion and
  streaming against the Messages API, model discovery, API-key
  authentication, and error translation into domain error kinds.
- Automatic authentication on start (`core.App.EnsureActive`): restores
  a stored session or runs the provider's preferred login flow without
  an explicit `auth login`.
- Retrying HTTP transport (`transport.DoWithRetry`): transient network
  errors, 429, and 5xx responses retry with exponential backoff,
  honoring `Retry-After`.
- `config.Save` persists configuration changes, used by setup and the
  `/model` command.

- Provider-independent authentication framework (`internal/auth`):
  authentication methods (API key, browser OAuth, device code, none),
  per-method flow contracts, an `auth.Manager` driving login, logout,
  session discovery, validation, and refresh, and `slickcode auth
  login|logout|status` commands.
- Secure credential storage on the OS-native keyring
  (`internal/auth/keyring`), with an in-memory store for tests. Secrets
  use a dedicated type whose `fmt` and `slog` output is always
  redacted.
- Provider lifecycle management (`provider.Lifecycle`): activation with
  session resolution and automatic refresh, deactivation, and health
  checking through optional `Activator`, `Deactivator`, and
  `HealthChecker` hooks, plus the `provider.Authenticator` contract for
  advertising authentication methods.
- First unit test suites: authentication manager, secret redaction, and
  provider lifecycle.

- Provider-agnostic domain model in `pkg/types`: multi-part messages
  (text, reasoning, images, files, tool calls, tool results),
  conversations, tools, models with discoverable capability sets, usage,
  responses, stream events, embeddings, image generation, and a
  classified error type (`types.Error` / `types.ErrorKind`).
- Provider operation contracts in `internal/provider`: optional
  `Completer`, `Streamer`, `Embedder`, and `ImageGenerator` interfaces
  discovered by type assertion, with a generic
  `provider.Capability[T]` lookup helper returning typed
  unsupported-capability errors.

- Initial project scaffolding: module layout, CLI entrypoint, configuration
  loading, and the provider extension point.
- Core runtime: application bootstrap and dependency injection
  (`internal/core`), configuration validation, structured logging
  (`internal/logging`), an injectable terminal IO surface, a
  constructor-based provider registry, and process lifecycle management
  with graceful shutdown on interrupt/terminate (`internal/runtime`).

### Changed

- Bootstrap now grants the `write` permission alongside `read`, so the
  editing tools are available; the permission policy remains the single
  authority over tool access.
- `auth.Credential` now records the authentication method, expiry, and
  refresh token, and its secret fields use the redacting `auth.Secret`
  type instead of plain strings.
- `types.Message` content is now an ordered list of parts instead of a
  single string; `types.NewTextMessage` covers the plain-text case.
- `provider.Provider` gained `Models(ctx)` for model discovery; registry
  and configuration errors are now classified `types.Error` values.
- `internal/provider`'s registry is now an instantiated `Registry` type
  instead of package-level global state; `Register` returns an error
  instead of panicking on a duplicate name.
- `internal/storage` now exposes a constructed `Paths` type instead of
  bare package functions.
- `internal/config.Load` takes an explicit file path instead of resolving
  it internally via `internal/storage`, removing that dependency.

### Fixed

- `slickcode` no longer exits silently with success when the provider
  reports no models and none is configured; it now explains the
  problem.
- `/history` no longer splits a multi-byte character when truncating
  long messages.
