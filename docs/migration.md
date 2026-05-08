# Enno Migration Guide

This guide records SDK-facing changes that may require application updates.

## v0.x Compatibility

Enno is still in the `v0.x` release series. The project prefers additive APIs and
keeps existing constructors and methods when practical, but breaking changes can
still happen before `v1.0.0`. When a breaking change is necessary, it should be
listed here with the old API, the new API, and the reason.

## v0.8.0 Explicit Session API

Breaking change:

- Removed `Agent.Run(ctx, input) (string, error)`.
- Removed `Agent.RunDetailed(ctx, input)` and `Agent.RunSession(ctx, session, input)`.
- Removed `Agent.Messages()` and `Agent.Reset()`; session state now lives only in `Session`.

Migration:

- Create a `Session` and call `Agent.Run(ctx, session, input)`.
- Read final text from `RunResult.Content`.
- Read or persist conversation history from `session.Messages`.
- Use `Agent.RunStream(ctx, session, input, handler)` for streaming.

Example:

```go
session := &enno.Session{}
result, err := agent.Run(ctx, session, "summarize this repository")
if err != nil {
    return err
}
fmt.Println(result.Content)
```

Reason:

- The SDK now uses one explicit run path with structured results and no hidden agent history.

## Current SDK APIs

- `Agent.Run` returns `RunResult` with messages, usage, rounds, stop reason, and duration.
- `Session` lets services load, persist, clone, and run explicit conversation state.
- `sdk.NewAgent` is the supported high-level entry point for built-in tool configuration and permissions.
- `NewStructuredTool` and `ToolResult` preserve model-visible content separately from metadata and tool error state.
- `SchemaObject` and `NewTypedToolFromSchema` reduce hand-written JSON schema maps.
- `Config.Options` and `RequestOptions` provide provider-neutral generation options.
- `Config.Policies` and `Config.Hooks` expose control points without replacing `EventHandler`.
- `Agent.RunStream` and optional `StreamProvider` add streaming support while keeping non-streaming providers valid.

## v0.9.0 Built-In Tool Configuration

Breaking change:

- Public `tools/*` built-in tool packages were removed from the SDK surface.
- Built-in tool implementations now live under `internal/builtintools/*`.

Migration:

- Use `sdk.NewAgent` with `sdk.Config.BuiltinTools` to enable and configure built-in tools.
- Use `sdk.ToolPermissions` with `AllowedTools` and `DisallowedTools` to restrict execution.
- Continue using root `enno.NewTool`, `enno.NewTypedTool`, and `enno.NewStructuredTool` for custom tools.

Example:

```go
agent, err := sdk.NewAgent(sdk.Config{
    Provider: provider,
    BuiltinTools: sdk.BuiltinTools{
        Filesystem: &sdk.FilesystemTool{Root: ".", Read: true, Write: false},
        Grep:       &sdk.GrepTool{Root: "."},
        Glob:       &sdk.GlobTool{Root: "."},
    },
    Permissions: sdk.ToolPermissions{
        DisallowedTools: []string{"bash", "write_file", "edit_file"},
    },
})
```

Reason:

- SDK users configure built-in capabilities declaratively instead of importing implementation packages directly.

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

- Enable `sdk.BuiltinTools.TaskGraph` and use the `task_create`, `task_update`, `task_list`, and `task_get` tools.

Reason:

- The task graph model supports persistent, structured task state instead of a flat todo list.
