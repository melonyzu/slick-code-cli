Contributing to Slick Code CLI

thank you for your interest in contributing.

Getting Started

git clone https://github.com/melonyzu/slick-code-cli.git
cd slick-code-cli
make build
make test

## Development Workflow

1. Open an issue before starting significant work so the approach can be discussed.
2. Create a branch from "main".
3. Keep commits focused and use clear commit messages.
4. Run "make check" before opening a pull request.
5. Open a pull request describing the motivation and implementation.

## Code Style

- Follow idiomatic Go
- Run "gofmt" before committing.
- Prefer the standard library where practical.
- Keep packages focused on a single responsibility.
- Add GoDoc comments for all exported identifiers.
- Remove unused code before submitting.
- Avoid placeholder implementations and unfinished work.

Provider Integrations

New providers should be implemented as separate packages under "internal/provider".

Providers must integrate through the existing provider interfaces and shared domain model. Provider-specific types, errors, and authentication details should remain inside the provider package.

Reuse the existing authentication, configuration, logging, runtime, and tool infrastructure rather than introducing parallel implementations.

Reporting Issues

Please use GitHub Issues for bug reports and feature requests.

For bug reports, include:

- the Slick Code CLI version,
- your operating system,
- steps to reproduce,
- expected behavior,
- actual behavior, and
- relevant logs or error messages, if available

for security issues, please follow the process described in SECURITY.md