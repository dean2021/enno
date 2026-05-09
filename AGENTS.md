# Repository Guidelines

## Purpose

This file gives Codex and other coding agents the same project rules captured in
`CLAUDE.md`. Keep both files synchronized when architecture, commands, public
APIs, or safety rules change.

## Project Structure & Module Organization

Enno is a Go module that ships both a provider-neutral SDK and an installable CLI.
The root `enno` package contains the public API (`Agent`, `Session`, `RunResult`,
`Config`, `Provider`, `Request`, `Response`, `RequestOptions`, `Message`, `Tool`,
`ToolCall`). The `sdk` package assembles built-in tools, custom tools, and
permissions. Provider adapters live in `provider/openai` and `provider/anthropic`.
Built-in tool implementations live under `internal/builtintools/*` for task
graph, filesystem, shell, grep, glob, fetch_url, subagent, load_skill, and
compact; do not expose new public `tools/*` packages. CLI prompt assembly lives
in `internal/systemprompt`, and project-level prompt loading lives in
`internal/projectrules`. CLI-only code belongs in `cmd/enno` and `internal/*`
(`cliconfig`, `cliui`, `history`, `httpproxy`). Examples are in `examples/*`;
design, usage, release, and migration docs are in `docs/`.

Keep dependency direction clean:

```text
cmd/enno -> internal/cliconfig -> sdk + provider/*
sdk -> enno + internal/builtintools/*
cmd/enno -> internal/cliui -> enno
provider/* -> enno
enno -> standard library only
```

## Build, Test, and Development Commands

- `make help`: list available project commands.
- `make version`: print the current `VERSION`.
- `make fmt`: run `gofmt -w .` across Go sources.
- `make tidy`: run `go mod tidy`.
- `make test`: run `go test ./...`.
- `make install`: install the local CLI from `./cmd/enno`.
- `make verify`: format, tidy, test, and install; run this before commits.
- `make release-check`: validate `VERSION`, `CHANGELOG.md`, tests, and CLI install.
- `go run ./examples/sdk_walkthrough`: run the complete offline SDK walkthrough.
- `go run ./examples/simple_agent`: run an example locally.
- `go run ./examples/custom_tool` and `go run ./examples/anthropic`: check SDK examples.

## Coding Style & Naming Conventions

Use idiomatic Go: simple names, small interfaces, explicit errors, and standard
formatting. Prefer existing mature Go libraries over hand-rolled
implementations for common functionality; implement in-house only when
available libraries do not meet Enno's needs. Preserve module path
`github.com/dean2021/enno` and semantic
versioning. The root package must stay provider-neutral: do not import OpenAI,
Anthropic, CLI config, or built-in tools, and do not expose provider SDK types.
Do not read env vars or `~/.enno/config.yaml` from the root package. CLI provider
configuration comes from YAML, not `ENNO_*`; only CLI behavior such as mouse
capture may read env in `internal/cliconfig`. Do not put Agent loop logic in
`cmd/enno`; the CLI creates an explicit `enno.Session`, calls `Agent.Run`, and
passes that session to `internal/cliui`. Tool names should be lowercase or
snake_case (`grep`, `glob`, `fetch_url`, `task_create`, `load_skill`).
CLI prompt text should be assembled from named sections, with project rules
loaded separately from prompt assembly. The SDK must not define a default agent
identity; applications define identity and custom prompt context through
`SystemPrompt` and `SystemPromptSections`.

## Public API & Extension Points

Prefer small stable interfaces: `Provider.Complete(ctx, enno.Request)`,
`Agent.Run(ctx, session, input)`, `Agent.RunStream(ctx, session, input, handler)`,
`enno.NewTool`, `enno.NewTypedTool[T]`, `enno.NewTypedToolFromSchema[T]`, and
`enno.NewStructuredTool`. Use `sdk.SystemPromptSection` for application-owned
named prompt sections such as Identity, Rules, Domain Context, or Output Style;
do not expose `internal/systemprompt.Section` as public API. Optional extension points include
`StreamProvider.Stream`, `Config.Hooks`, and `Config.Policies`.

Add providers under `provider/<name>`; they should own their config, construct the
SDK client internally, convert `enno` requests/responses, and never execute local
tools. Add built-in tools under `internal/builtintools/<name>` and expose them
through `sdk.BuiltinTools`; custom tools should use typed or structured root
helpers unless raw JSON is required.

## Documentation & Release Notes

When adding or changing public API, update `README.md`, `docs/usage-sdk.md`,
`docs/usage-cli.md`, and relevant examples. Keep `README.md` concise and move
deep details to `docs/`. For releases, update `VERSION` and `CHANGELOG.md`
together; `VERSION` must not include a leading `v`.

## Testing Guidelines

Write focused Go unit tests with names like `TestAgentRun...` or
`TestParse...`. Place tests near the package under test. Cover provider option
translation, tool validation, session behavior, CLI config parsing, and edge cases
for filesystem or shell safety when touched. Run `make test` while iterating and
`make verify` before finalizing code changes.

## Commit & Pull Request Guidelines

Recent history uses concise Conventional Commit-style messages such as
`docs: refresh post-v0.8 SDK guidance`, `feat: improve SDK ergonomics`, and
`feat!: require explicit SDK sessions`. Use the same pattern; reserve `!` for
breaking SDK changes. Pull requests should describe the change, list verification
commands run, link related issues when available, and include screenshots only for
visible TUI changes.

## Security & Configuration Tips

Do not hard-code API keys. CLI provider credentials come from YAML config, not
`ENNO_*` environment variables. Treat `sdk.ShellTool` and filesystem access as
opt-in capabilities and keep roots, workdirs, timeouts, and output limits scoped.
Do not log secrets from environment variables or provider configs. Event handlers
may expose observable model/tool metadata, but must not claim to expose hidden
chain-of-thought.
