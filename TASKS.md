# Built-In Tool Configuration Refactor Plan

## Goal

让 SDK 用户像 Claude Code SDK 一样通过配置启用、禁用和限制内置工具，而不是直接导入 `tools/filesystem`、`tools/shell`、`tools/grep` 等内置工具包。根包 `enno` 继续保持 provider-neutral，不导入 provider SDK、CLI 配置或内置工具实现。

## Design Direction

- 保留根包 `github.com/dean2021/enno` 作为核心 Agent runtime。
- 新增高层装配包，暂定 `github.com/dean2021/enno/sdk`，负责 provider、内置工具、权限和 session 友好配置。
- 将当前公开的内置工具实现迁移到 `internal/builtintools/*`。
- 对外通过 `sdk.Config` 暴露能力开关、工具参数和权限策略。
- `enno.Tool`、`enno.NewTool`、`NewTypedTool`、`NewStructuredTool` 继续作为自定义工具扩展点。
- 这是 breaking change，不保留旧公开 `tools/*` 内置工具导入路径。

## Proposed Public API

```go
agent, err := sdk.NewAgent(sdk.Config{
    Provider: provider,
    SystemPrompt: "You are a helpful coding agent.",
    BuiltinTools: sdk.BuiltinTools{
        Filesystem: &sdk.FilesystemTool{
            Root: ".",
            Read: true,
            Write: false,
            MaxOutputChars: 50000,
        },
        Grep: &sdk.GrepTool{Root: "."},
        Glob: &sdk.GlobTool{Root: "."},
        Shell: nil,
        TaskGraph: &sdk.TaskGraphTool{Root: "."},
    },
    Permissions: sdk.ToolPermissions{
        Mode: sdk.PermissionDeny,
        AllowedTools: []string{"read_file", "grep", "glob", "task_create", "task_update", "task_list", "task_get"},
        DisallowedTools: []string{"bash", "write_file", "edit_file"},
    },
    CustomTools: []enno.Tool{customTool},
})
```

## Phase 1: API Shape and Package Boundary [done]

- [x] Decide final high-level package name: `sdk`, `agent`, or `runtime`.
- [x] Define `sdk.Config`, `BuiltinTools`, tool config structs, and `ToolPermissions`.
- [x] Define permission modes:
  - [x] `PermissionAsk` for host-controlled approval hooks.
  - [x] `PermissionAllow` for explicitly allowed tools.
  - [x] `PermissionDeny` for no-prompt deny behavior.
- [x] Keep `sdk.Config` separate from `enno.Config` while passing through core fields such as `Provider`, `SystemPrompt`, `Options`, `Hooks`, `Policies`, `EventHandler`, `Compaction`, and `MaxToolRounds`.
- [x] Document that root `enno` remains standard-library-only.

## Phase 2: Move Built-In Tool Implementations Internal [done]

- [x] Move `tools/taskgraph` to `internal/builtintools/taskgraph`.
- [x] Move `tools/filesystem` to `internal/builtintools/filesystem`.
- [x] Move `tools/shell` to `internal/builtintools/shell`.
- [x] Move `tools/grep` to `internal/builtintools/grep`.
- [x] Move `tools/glob` to `internal/builtintools/glob`.
- [x] Move `tools/subagent` to `internal/builtintools/subagent`.
- [x] Move `tools/loadskill` to `internal/builtintools/loadskill`.
- [x] Move `tools/compact` to `internal/builtintools/compact`.
- [x] Preserve package tests during the move and update import paths.

## Phase 3: Build the High-Level SDK Assembler [done]

- [x] Implement `sdk.NewAgent(config sdk.Config) (*enno.Agent, error)`.
- [x] Assemble built-in tools only from `config.BuiltinTools`.
- [x] Merge `CustomTools` after built-ins and keep duplicate-name validation through `enno.NewAgent`.
- [x] Add default tool config behavior consistent with current CLI defaults.
- [x] Ensure `subagent` receives the same allowed child tool set but never includes itself recursively.
- [x] Ensure `compact` is registered only when compaction is enabled or explicitly requested.

## Phase 4: Permission Layer [done]

- [x] Implement a permission hook that enforces `AllowedTools` and `DisallowedTools`.
- [x] Define deterministic conflict behavior: `DisallowedTools` wins over `AllowedTools`.
- [x] Support exact tool names first; consider pattern support later only if needed.
- [x] Return model-visible denial messages using existing hook denial semantics.
- [x] Add tests for allow-only, deny-only, conflict, unknown tool, and shell deny cases.

## Phase 5: CLI Migration [done]

- [x] Update `internal/cliconfig` to produce `sdk.Config` or equivalent high-level tool config.
- [x] Replace direct imports of public `tools/*` packages in CLI assembly.
- [x] Preserve current CLI YAML fields: `shell`, `filesystem`, `grep`, `glob`, `task_graph`, `subagent`, `skills_dir`, `skills_extra_dirs`, `compaction`.
- [x] Add optional YAML permission fields:
  - [x] `allowed_tools`
  - [x] `disallowed_tools`
  - [x] `permission_mode`
- [x] Keep CLI provider credentials YAML-only and avoid `ENNO_*` provider fallback.

## Phase 6: Examples and Documentation [done]

- [x] Update `examples/sdk_walkthrough` to use the high-level SDK API.
- [x] Update or remove examples that import `tools/*` directly.
- [x] Update `README.md` package usage snippet.
- [x] Update `docs/usage-sdk.md` with built-in tool configuration and permissions.
- [x] Update `docs/usage-cli.md` with permission YAML examples.
- [x] Update `docs/design.md`, `docs/migration.md`, `CLAUDE.md`, and `AGENTS.md`.
- [x] Add changelog entry under `Unreleased` marking the public tool-package removal as breaking.

## Phase 7: Cleanup of Public Surface [done]

- [x] Remove public `tools/*` package directories or leave no public Go packages under those paths.
- [x] Verify no README, docs, examples, or tests recommend importing `tools/*`.
- [x] Decide whether `tools` should remain only as internal implementation detail in repository layout docs.
- [x] Keep custom tool APIs in root `enno` documented as the intended extension path.

## Phase 8: Validation

- [x] Run `go test ./...`.
- [x] Run `go test ./examples/...`.
- [x] Run `make verify`.
- [x] Run CLI smoke tests for:
  - [x] default interactive assembly.
  - [x] `enno run`.
  - [x] shell disabled.
  - [x] filesystem disabled.
  - [x] permission denied tool call.
- [x] Run `git diff --check`.

## Acceptance Criteria

- [x] SDK users can enable, disable, and configure built-in tools without importing internal tool packages.
- [x] SDK users can restrict tool execution with allow/deny configuration.
- [x] Custom tools remain supported through root `enno.Tool`.
- [x] Root `enno` keeps clean dependency direction and remains provider-neutral.
- [x] CLI behavior remains compatible unless permission fields are explicitly configured.
- [x] Documentation clearly states the new supported SDK path and migration from public `tools/*` imports.
