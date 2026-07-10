## Slick Code CLI

Slick Code CLI is a terminal-based coding assistant that provides a consistent interface for working with multiple AI providers.

Rather than using a different workflow for each provider, Slick Code exposes a single command-line interface while allowing each provider to use its own authentication, models, and capabilities.

## Supported providers include:

- Anthropic
- OpenAI
- Ollama
- OpenRouter

## Additional providers can be added without changing the CLI or runtime architecture.

«Status: Pre-release. Core functionality is implemented and under active development. See the "Roadmap" (#roadmap) for planned work.»

## Installation

The first public release has not been published yet.

## To build from source:

git clone https://github.com/melonyzu/slick-code-cli.git
cd slick-code-cli
make build

## Usage

Run the CLI without arguments to start an interactive session.

slickcode

On first launch, Slick Code guides you through provider setup and model selection. After setup, the interactive assistant starts immediately.

Useful commands include:

slickcode auth login
slickcode auth logout
slickcode auth status
slickcode version

Within the interactive session, use "/help" to list available commands.

Configuration is stored in the platform's standard configuration directory and can be overridden with "SLICKCODE_*" environment variables. Credentials are stored using the operating system's secure credential store.

