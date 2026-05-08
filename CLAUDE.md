# CLAUDE.md

This file gives Claude and other coding agents the context needed to work on Enno safely.

## Project Overview

Enno is a Go agent framework that can be used both as:

- a library package: `github.com/dean2021/enno`
- an installable CLI: `github.com/dean2021/enno/cmd/enno`

The framework core is provider-neutral. It owns the Agent loop, message history, tool dispatch, and public interfaces. Concrete model SDKs live only in provider subpackages.

## Architecture

Important packages:

- `enno`: public core API, including `Agent`, `Config`, `Provider`, `Request`, `Response`, `Message`, `Tool`, and `ToolCall`.
- `provider/openai`: OpenAI Chat Completions compatible provider.
- `provider/anthropic`: Anthropic Messages API provider.
- `tools/taskgraph`: optional persistent task graph (`task_create`, etc.); library default `Root/.tasks/`, CLI uses `~/.enno/tasks/<session_id>/`.
- `tools/filesystem`: optional filesystem tools scoped by `filesystem.Config.Root`.
- `tools/shell`: optional shell tool scoped by `shell.Config.Workdir`, timeout, and denylist.
- `tools/grep`: optional `grep` tool (ripgrep `rg` subprocess); scoped by `grep.Config.Root`; requires `rg` on PATH.
- `tools/glob`: optional `glob` tool (`rg --files` subprocess); scoped by `glob.Config.Root`; requires `rg` on PATH.
- `tools/subagent`: optional `subagent` tool (isolated child agent).
- `tools/loadskill`: optional `load_skill` tool and `SKILL.md` directory loader (`LoadDirs` merges multiple roots).
- `internal/cliui`: CLI-only terminal UI and non-terminal fallback.
- `internal/cliconfig`: CLI-only flag/env parsing.
- `cmd/enno`: thin installable CLI entrypoint.
- `examples`: small examples for package usage.
- `docs`: design, SDK usage, CLI usage, and release documentation.

Keep dependency direction clean:

```text
cmd/enno -> internal/cliconfig -> enno + provider/* + tools/*
cmd/enno -> internal/cliui -> enno
provider/* -> enno
tools/* -> enno
enno -> standard library only
```

The root `enno` package must not import OpenAI, Anthropic, CLI config, or built-in tool packages.

## Common Commands

Show available commands:

```sh
make help
```

Run all checks:

```sh
make verify
```

Show current project version:

```sh
make version
```

Run release checks:

```sh
make release-check
```

Format code:

```sh
make fmt
```

Tidy modules:

```sh
make tidy
```

Verify CLI installation:

```sh
make install
```

Run tests only:

```sh
make test
```

Run examples:

```sh
go run ./examples/simple_agent
go run ./examples/custom_tool
go run ./examples/anthropic
```

## Development Rules

- Preserve the module path `github.com/dean2021/enno`.
- Preserve semantic versioning. Update `VERSION` and `CHANGELOG.md` together for releases.
- Keep the root package as the stable public API.
- Keep code cohesive and loosely coupled. Prefer clear package boundaries, small interfaces, and elegant implementations over ad hoc wiring.
- Follow idiomatic Go style. Prefer simple names, small interfaces, explicit errors, standard formatting, and package layouts that match Go conventions.
- Do not expose OpenAI or Anthropic SDK types from the root package.
- Do not add environment variable reads to the root package. CLI env/flag parsing belongs in `internal/cliconfig`.
- Keep CLI config file parsing in `internal/cliconfig`; the root package must not read `~/.enno/config.yaml`.
- CLI provider configuration must come from `config.yaml`, not `ENNO_*` environment variables.
- Do not expose REPL/TUI helpers as public SDK packages. CLI UI belongs under `internal/cliui`.
- Do not put Agent loop logic in `cmd/enno`; the CLI should call `enno.Agent` directly for one-shot execution and use `internal/cliui` for interactive mode.
- Do not introduce package-level mutable state for tools. Tool state should belong to a tool instance.
- Treat shell and filesystem tools as opt-in capabilities.
- Keep provider packages focused on protocol conversion and SDK calls. Providers should not execute local tools.
- Event handlers may expose observable model/tool execution metadata, but must not claim to expose hidden chain-of-thought.
- Fully test new functionality before considering it complete. Add focused tests for new behavior and run enough verification to avoid regressions.
- Run `make verify` after code changes. It formats code, tidies modules, runs tests, and verifies CLI installation.

## Public API Guidelines

Prefer small, stable interfaces:

- `Provider.Complete(ctx, enno.Request) (enno.Response, error)`
- `Agent.Run(ctx, input) (string, error)`
- `enno.NewTool` for raw JSON handlers
- `enno.NewTypedTool[T]` for typed tool arguments

When adding new public API, update:

- `README.md`
- `docs/usage-sdk.md`
- `docs/usage-cli.md`
- examples if the API changes user-facing behavior

## Adding a Provider

Add a new package under `provider/<name>`.

The provider should:

1. define its own `Config`
2. construct the SDK client internally
3. implement `enno.Provider`
4. convert `enno.Message` and `enno.Tool` into the provider SDK format
5. convert provider responses back into `enno.Response`

Do not modify `Agent` unless the common provider contract is insufficient.

## Adding a Tool

Add a new package under `tools/<name>` if it is broadly reusable.

Tools should return `enno.Tool` or `[]enno.Tool` and should keep any mutable state inside the returned tool instance or an internal struct.

Use `enno.NewTypedTool[T]` unless raw JSON handling is needed.

## Documentation

Primary documentation:

- `README.md`: project overview and quick start
- `docs/design.md`: architecture and design notes
- `docs/usage-sdk.md`: SDK and package usage
- `docs/usage-cli.md`: CLI usage
- `docs/release.md`: testing, versioning, and release workflow

Keep README concise and move deeper explanations to `docs`.

## Safety Notes

- Be conservative with shell execution changes.
- Keep filesystem access constrained by a configured root.
- Never hard-code API keys.
- Do not log secrets from environment variables or provider configs.
