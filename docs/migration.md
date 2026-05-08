# Enno Migration Guide

This guide records SDK-facing changes that may require application updates.

## v0.x Compatibility

Enno is still in the `v0.x` release series. The project prefers additive APIs and
keeps existing constructors and methods when practical, but breaking changes can
still happen before `v1.0.0`. When a breaking change is necessary, it should be
listed here with the old API, the new API, and the reason.

## Current SDK Additions

These changes are additive and do not require existing callers to migrate:

- `Agent.RunDetailed` returns `RunResult` with messages, usage, rounds, stop reason, and duration.
- `Session` and `Agent.RunSession` let services load, persist, clone, and run explicit conversation state.
- `NewStructuredTool` and `ToolResult` preserve model-visible content separately from metadata and tool error state.
- `SchemaObject` and `NewTypedToolFromSchema` reduce hand-written JSON schema maps.
- `Config.Options` and `RequestOptions` provide provider-neutral generation options.
- `Config.Policies` and `Config.Hooks` expose control points without replacing `EventHandler`.
- `Agent.RunStream` and optional `StreamProvider` add streaming support while keeping non-streaming providers valid.

## v0.6.0 Tool Name Renames

Breaking change:

- `tools/grep` registered tool name changed from `Grep` to `grep`.
- `tools/glob` registered tool name changed from `Glob` to `glob`.
- `tools/subagent` registered tool name changed from `task` to `subagent`.

Migration:

- If prompts, tests, or custom provider fixtures refer to the old names, update them to the new lowercase names.
- If code checks `Tool.Name`, update expected values to `grep`, `glob`, or `subagent`.

Reason:

- Built-in tool names now use a consistent snake_case/lowercase convention.

## v0.5.0 Task Tool Replacement

Breaking change:

- The old `tools/todo` package and `todo` tool were removed.

Migration:

- Use `tools/taskgraph` and the `task_create`, `task_update`, `task_list`, and `task_get` tools.

Reason:

- The task graph model supports persistent, structured task state instead of a flat todo list.
