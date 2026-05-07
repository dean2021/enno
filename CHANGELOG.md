# Changelog

All notable changes to Enno will be documented in this file.

The project follows [Semantic Versioning](https://semver.org/). While the public API is still evolving, releases use the `v0.x.y` series.

## [Unreleased]

## [0.3.0] - 2026-05-07

### Changed

- Replaced the CLI terminal UI dependency from `tui-go` to `tview`.

### Added

- Added Agent events and CLI observability panels for model calls, tool calls, tool results, and token usage.

## [0.2.0] - 2026-05-07

### Changed

- Changed CLI provider configuration to read model, API key, base URL, and max tokens from `config.yaml` only.
- Moved CLI REPL/TUI implementation from the public `runner` package to `internal/cliui`.
- Removed `workdir` and `prompt` from CLI YAML configuration.
- Changed the CLI interactive mode to use a `tui-go` terminal UI.
- Removed CLI support for `ENNO_*` provider configuration environment variables.

### Added

- Added automatic creation of a commented `~/.enno/config.yaml` template when the default CLI config file is missing.
- Added internal CLI UI tests for the non-terminal REPL fallback.
- Added CLI YAML config loading from `~/.enno/config.yaml` and `--config`.
- Added CLI config tests for YAML-only provider configuration.
- Added CLI configuration tests for required model and OpenAI-compatible base URL settings.

## [0.1.0] - 2026-05-07

### Added

- Introduced the provider-neutral `enno` core package with `Agent`, `Config`, `Provider`, `Tool`, and message types.
- Added OpenAI-compatible and Anthropic provider packages.
- Added optional built-in tools for todo tracking, filesystem access, and shell execution.
- Added reusable `runner.REPL` and `runner.Once` helpers.
- Added installable CLI under `cmd/enno`.
- Added usage, design, README, and agent guidance documentation.
- Added repository verification through `make verify`.

[Unreleased]: https://github.com/dean2021/enno/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/dean2021/enno/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/dean2021/enno/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/dean2021/enno/releases/tag/v0.1.0
