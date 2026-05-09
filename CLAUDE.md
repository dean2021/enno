# CLAUDE.md

This file gives Claude and other coding agents the context needed to work on Enno safely.

## Project Overview

Enno is a Go agent framework that can be used both as:

- a library package: `github.com/dean2021/enno`
- an installable CLI: `github.com/dean2021/enno/cmd/enno`

The framework core is provider-neutral. It owns the Agent loop, explicit `Session` state model, tool dispatch, and public interfaces. Concrete model SDKs live only in provider subpackages.

## Architecture

Important packages:

- `enno`: public core API, including `Agent`, `Session`, `RunResult`, `Config`, `Provider`, `Request`, `Response`, `RequestOptions`, `Message`, `Tool`, and `ToolCall`.
- `sdk`: high-level SDK assembler for built-in tool configuration, custom tools, and tool permissions.
- `provider/openai`: OpenAI Chat Completions compatible provider.
- `provider/anthropic`: Anthropic Messages API provider.
- `internal/builtintools/*`: internal implementations for task graph, filesystem, shell, grep, glob, fetch_url, subagent, load_skill, and compact.
- `internal/cliui`: CLI-only terminal UI and non-terminal fallback.
- `internal/cliconfig`: CLI-only flag and YAML config parsing.
- `internal/history`: CLI history recorder and reader.
- `internal/httpproxy`: CLI proxy helper for HTTP clients.
- `cmd/enno`: thin installable CLI entrypoint.
- `examples`: small examples for package usage.
- `docs`: design, SDK usage, CLI usage, and release documentation.

Keep dependency direction clean:

```text
cmd/enno -> internal/cliconfig -> sdk + provider/*
sdk -> enno + internal/builtintools/*
cmd/enno -> internal/cliui -> enno
provider/* -> enno
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
go run ./examples/sdk_walkthrough
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
- Prefer existing mature Go libraries over hand-rolled implementations for common functionality; implement in-house only when available libraries do not meet Enno's needs.
- Do not expose OpenAI or Anthropic SDK types from the root package.
- Do not add environment variable reads to the root package. CLI env/flag parsing belongs in `internal/cliconfig`.
- Keep CLI config file parsing in `internal/cliconfig`; the root package must not read `~/.enno/config.yaml`.
- CLI provider configuration must come from `config.yaml`, not `ENNO_*` environment variables.
- Do not expose REPL/TUI helpers as public SDK packages. CLI UI belongs under `internal/cliui`.
- Do not put Agent loop logic in `cmd/enno`; the CLI should create an explicit `enno.Session`, call `Agent.Run` for one-shot execution, and pass the same session into `internal/cliui` for interactive mode.
- Do not introduce package-level mutable state for tools. Tool state should belong to a tool instance.
- Treat shell and filesystem tools as opt-in capabilities.
- Keep provider packages focused on protocol conversion and SDK calls. Providers should not execute local tools.
- Event handlers may expose observable model/tool execution metadata, but must not claim to expose hidden chain-of-thought.
- Fully test new functionality before considering it complete. Add focused tests for new behavior and run enough verification to avoid regressions.
- Run `make verify` after code changes. It formats code, tidies modules, runs tests, and verifies CLI installation.

## Public API Guidelines

Prefer small, stable interfaces:

- `Provider.Complete(ctx, enno.Request) (enno.Response, error)`
- `Agent.Run(ctx, session, input) (enno.RunResult, error)`
- `Agent.RunStream(ctx, session, input, handler)` for streaming callers
- `enno.NewTool` for raw JSON handlers
- `enno.NewTypedTool[T]` and `enno.NewTypedToolFromSchema[T]` for typed tool arguments
- `enno.NewStructuredTool` when tool metadata/error state must be preserved

Optional extension points:

- `StreamProvider.Stream(ctx, enno.Request) (enno.Stream, error)`
- `Config.Hooks` for provider/tool call interception
- `Config.Policies` for loop-stage behaviors

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

Add built-in tool implementations under `internal/builtintools/<name>` and expose them through `sdk.BuiltinTools` config, not a public `tools/*` package.

Custom SDK tools should return `enno.Tool` or `[]enno.Tool` and should keep any mutable state inside the returned tool instance or an internal struct.

Use `enno.NewTypedTool[T]` / `enno.NewTypedToolFromSchema[T]` unless raw JSON handling is needed. Use `NewStructuredTool` when returning metadata with model-visible content.

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
