# Changelog

All notable changes to Slick Code CLI are documented here.

## [Unreleased]

### Added

- Added support for Anthropic, OpenAI, and Ollama.
- Built the interactive terminal experience with streaming responses and slash commands.
- Added first-run setup for choosing a provider, signing in, and selecting a default model.
- Added project context generation with `.gitignore` support and incremental refresh.
- Added built-in tools for reading files, editing files, and common Git operations.
- Added a safe editing engine with previews, rollback support, and conflict detection.
- Added secure credential storage using the operating system's credential store.
- Added automatic provider discovery and model selection.
- Added retry handling for temporary network failures.
- Added test coverage across the core packages.

### Changed

- Simplified configuration loading.
- Improved provider registration and capability detection.
- Unified tool execution behind a single manager.
- Improved project discovery and workspace handling.
- improved error handling across providers.

### Fixed

- Fixed model selection when no default model is configured.
- Fixed message being cut off in `/history`.
- Fixed several edge cases around file editing and workspace detection.
