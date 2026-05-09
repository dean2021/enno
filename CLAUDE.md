# Repository Guidelines

## Purpose

This file gives Codex and other coding agents the same project rules captured in
`CLAUDE.md`. Keep both files synchronized when architecture, commands, public
APIs, or safety rules change.

## Project Structure & Module Organization

Enno is a Go SDK module. The root `enno` package contains the public API:
`Agent`, `Session`, `RunResult`, `Config`, `Provider`, `Request`, `Response`,
`RequestOptions`, `Message`, `Tool`, and `ToolCall`. The root package also
provides `NewAgent`, `BuiltinTools`, `SystemPromptSection`, `ToolPermissions`,
and the built-in tool configuration types previously in the `sdk` package.
The `setup` package (blank-import `github.com/dean2021/enno/setup`) registers
built-in tool builders before `NewAgent` is called with `BuiltinTools`.
Provider adapters live in `provider/openai` and
`provider/anthropic`; provider-shared HTTP helpers live in
`provider/internal/httpproxy`. Built-in tools live under
`builtintools/*` for task graph, filesystem, shell, grep, glob,
fetch_url, subagent, load_skill, and compact. Runtime prompt helpers live in
`prompt`. Examples are in `examples/*`; design, SDK usage,
release, and migration docs are in `docs/`.

The CLI has moved to the standalone Godo project at
`../godo-coding-agent`. Do not reintroduce
`cmd/enno`, CLI config, TUI, history, coding-agent prompt, or project-rule loader
packages into this SDK repository.

Keep dependency direction clean:

```text
setup -> enno + builtintools/* + prompt
provider/* -> enno
provider/* -> provider/internal/httpproxy
enno -> prompt + standard library (no builtintools imports)
```

## Build, Test, and Development Commands

- `make help`: list available SDK commands.
- `make version`: print the current `VERSION`.
- `make fmt`: run `gofmt -w .` across Go sources.
- `make tidy`: run `go mod tidy`.
- `make test`: run `go test ./...`.
- `make examples`: compile example packages.
- `make verify`: format, tidy, and test the SDK module.
- `make release-check`: validate `VERSION`, `CHANGELOG.md`, and tests.
- `go run ./examples/sdk_walkthrough`: run the complete offline SDK walkthrough.
- `go run ./examples/simple_agent`: run an example locally.
- `go run ./examples/custom_tool` and `go run ./examples/anthropic`: check SDK examples.

## Coding Style & Naming Conventions

Use idiomatic Go: simple names, small interfaces, explicit errors, and standard
formatting. Prefer mature Go libraries over hand-rolled implementations for
common functionality; implement in-house only when available libraries do not
meet Enno's needs. Preserve module path `github.com/dean2021/enno` and semantic
versioning. The root package must stay provider-neutral: do not import OpenAI,
Anthropic, CLI config, built-in tools, or provider SDK types. Do not read env
vars, user home directories, YAML config files, git state, or CLI-branded paths
from the root package. Tool names should be lowercase or snake_case (`grep`,
`glob`, `fetch_url`, `task_create`, `load_skill`).

The SDK must not define a default agent identity. Applications define identity
and custom prompt context through `SystemPrompt` and `SystemPromptSections`; SDK
owned prompt additions must stay generic runtime capability sections. Put
tool-specific usage guidance in the relevant tool description rather than a
global system prompt section.

## Public API & Extension Points

Prefer small stable interfaces: `Provider.Complete(ctx, enno.Request)`,
`Agent.Run(ctx, session, input)`, `Agent.RunStream(ctx, session, input, handler)`,
`enno.NewTool`, `enno.NewTypedTool[T]`, `enno.NewTypedToolFromSchema[T]`, and
`enno.NewStructuredTool`. Use `enno.SystemPromptSection` for application-owned
named prompt sections such as Identity, Rules, Domain Context, or Output Style;
do not expose `prompt.Section` as public API. Optional extension
points include `StreamProvider.Stream`, `Config.Hooks`, and `Config.Policies`.

Add providers under `provider/<name>`; they should own their config, construct the
SDK client internally, convert `enno` requests/responses, and never execute local
tools. Add built-in tools under `builtintools/<name>` and expose them
through `enno.BuiltinTools`; custom tools should use typed or structured root
helpers unless raw JSON is required.

## Documentation & Release Notes

When adding or changing public API, update `README.md`, `docs/usage-sdk.md`, and
relevant examples. Keep `README.md` concise and move deep details to `docs/`.
For releases, update `VERSION` and `CHANGELOG.md` together; `VERSION` must not
include a leading `v`.

## Testing Guidelines

Write focused Go unit tests with names like `TestAgentRun...` or `TestParse...`.
Place tests near the package under test. Cover provider option translation, tool
validation, session behavior, permissions, compaction, and filesystem or shell
safety edge cases when touched. Run `make test` while iterating and `make verify`
before finalizing SDK changes.

## Commit & Pull Request Guidelines

Recent history uses concise Conventional Commit-style messages such as
`docs: refresh post-v0.8 SDK guidance`, `feat: improve SDK ergonomics`, and
`feat!: require explicit SDK sessions`. Use the same pattern; reserve `!` for
breaking SDK changes. Pull requests should describe the change, list verification
commands run, and link related issues when available.

## Security & Configuration Tips

Do not hard-code API keys. Treat `enno.ShellTool` and filesystem access as opt-in
capabilities and keep roots, workdirs, timeouts, and output limits scoped. Do not
log secrets from environment variables or provider configs. Event handlers may
expose observable model/tool metadata, but must not claim to expose hidden
chain-of-thought.
