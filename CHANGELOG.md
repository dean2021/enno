# Changelog

All notable changes to Enno will be documented in this file.

The project follows [Semantic Versioning](https://semver.org/). While the public API is still evolving, releases use the `v0.x.y` series.

## [Unreleased]

### Added

- `tools/taskgraph`: persistent DAG task plan (**`task_create`**, **`task_update`**, **`task_list`**, **`task_get`**). **CLI** stores JSON under **`~/.enno/tasks/<session_id>/`** (UUID v4 per run); library default remains **`Root/.tasks/`** when `TasksDir` is empty. CLI YAML **`task_graph`** / **`--no-task-graph`** (default: enabled). Replaces the removed `todo` tool.
- `tools/grep`: **`Grep`** tool (Claude Code–compatible name) running **`rg`** with path scope under `Config.Root`; CLI YAML `grep` / flag `--no-grep` (default: grep enabled). Requires [ripgrep](https://github.com/BurntSushi/ripgrep) installed.
- `tools/glob`: **`Glob`** tool (Claude Code–compatible name) listing files with **`rg --files`** under `Config.Root`; CLI YAML `glob` / flag `--no-glob` (default: glob enabled). Requires **ripgrep** on `PATH`.
- Optional **context compaction** (`Config.Compaction`): micro-trimming of older long tool results, automatic summarization when estimated input usage crosses a threshold (writes JSONL transcripts under `TranscriptDir`, default `~/.enno/transcripts` when enabled), and a manual `compact` tool (same summarization path; must be the only tool call in that turn). **Default off**; extra model calls and disk writes when enabled. Implementation lives in `compaction_impl.go` in the root package (a separate `compaction` subpackage would import-cycle with `enno`).
- `tools/compact`: registers the `compact` tool name for the runtime; CLI appends it when `compaction` is set in `config.yaml`.
- `tools/subagent`: optional `task` tool that runs a child `enno.Agent` with isolated history; CLI can enable it with `subagent: true` in `~/.enno/config.yaml` or disable with `--no-subagent`.
- `tools/loadskill`: load `SKILL.md` skills from a directory, inject short descriptions into the system prompt, and register `load_skill` for on-demand full text; CLI merges default `~/.enno/skills` with optional `skills_extra_dirs`, `skills_dir`, and `--skills-dir` (later roots override same skill name).

### Fixed

- TUI: use button-level mouse mode (not full motion/drag reporting) so the main transcript can be click-dragged for copy again; keep wheel scroll routing.

### Removed

- **`tools/todo`** and the **`todo`** tool: use **`task_*`** task graph tools instead. Agent “plan reminder” now tracks **`task_create` / `task_update` / `task_list` / `task_get`** and injects `<reminder>Update your task plan.</reminder>`.

### Changed

- **CLI**：首次自动创建的 `~/.enno/config.yaml` 模板默认包含 **`compaction.enabled: true`**（仍可改为 `false`）；库 API 仍为 `Compaction == nil` 时不启用压缩。
- **Compaction**：摘要提示改为 `<analysis>` / `<summary>` 结构并后处理为正文；支持 `ModelContextTokens` + buffer 阈值、`MicroCompactToolNames`、上一轮 API `InputTokens` 与估算取 max、摘要失败时半量重试、`SkipOnSummarizeError` 与同一 `Run` 内连续失败熔断；手动 `compact` 仍严格失败即报错。
- TUI: the main transcript scrolls only with the mouse wheel over that pane; keyboard paging keys no longer scroll it.

## [0.4.0] - 2026-05-08

### Added

- Save user input history to `~/.enno/history.jsonl` in JSONL format with display text, timestamp, project path, and session ID.
- TUI prompt supports Up/Down arrow keys to navigate input history with draft preservation.
- Load last 500 history entries on TUI startup for immediate navigation.
- Add `internal/history` package with `Recorder` (append-only writer) and `LoadRecent` (reader).
- Add `Project` and `SessionID` fields to CLI config for history tracking.
- Generate random session ID per CLI invocation.

### Changed

- Main view scrolling changed from Up/Down arrows to PgUp/PgDn, freeing Up/Down for input history navigation.
- Updated status bar hints to reflect new key bindings.

## [0.3.1] - 2026-05-07

### Fixed

- TUI: keep the main transcript following the end of the output when "follow latest" is enabled (avoid canceling tview track-end mode after scroll).

### Changed

- TUI: softer rounded borders, muted border and title colors, and consistent focus border glyphs between the transcript and prompt panes.
- TUI: animated transcript title while waiting on the model during network latency.
- TUI: use a distinct color for the `tool:` channel label versus the tool name in the conversation stream.

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

[Unreleased]: https://github.com/dean2021/enno/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/dean2021/enno/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/dean2021/enno/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/dean2021/enno/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/dean2021/enno/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/dean2021/enno/releases/tag/v0.1.0
