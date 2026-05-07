# Changelog

All notable changes to Enno will be documented in this file.

The project follows [Semantic Versioning](https://semver.org/). While the public API is still evolving, releases use the `v0.x.y` series.

## [Unreleased]

## [0.1.0] - 2026-05-07

### Added

- Introduced the provider-neutral `enno` core package with `Agent`, `Config`, `Provider`, `Tool`, and message types.
- Added OpenAI-compatible and Anthropic provider packages.
- Added optional built-in tools for todo tracking, filesystem access, and shell execution.
- Added reusable `runner.REPL` and `runner.Once` helpers.
- Added installable CLI under `cmd/enno`.
- Added usage, design, README, and agent guidance documentation.
- Added repository verification through `make verify`.

[Unreleased]: https://github.com/dean2021/enno/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/dean2021/enno/releases/tag/v0.1.0
