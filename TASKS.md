# Identity and Custom System Prompt Sections Plan

## Goal

将 Enno 的 system prompt 设计调整为更适合通用 SDK 的模型：SDK 不再默认定义
agent identity，identity 由应用层负责；同时 SDK 提供公开的自定义 section
扩展点，方便用户以结构化方式注入 Identity、Rules、Domain Context、Output Style
等内容。CLI 作为基于 SDK 构建的 coding agent 应用，应在 CLI 层定义自己的默认
identity 和 section 组合。

## Design Principles

- 根包 `enno` 继续保持 provider-neutral，只定义 runtime 和公共接口。
- `sdk` 负责能力组装，不默认声明“你是谁”；调用方负责定义 identity。
- CLI 可以有默认 coding-agent identity，但该默认值属于 CLI 应用，不属于 SDK。
- 公开 API 不暴露 `internal/systemprompt.Section`，避免把内部实现泄漏给 SDK 用户。
- 保留简单字符串入口，同时增加结构化 section 入口；用户可按复杂度选择。
- SDK 自动追加的内容应只描述启用能力，例如 skills/tool guidance，不覆盖用户 identity。

## Proposed Public API Shape

```go
type SystemPromptSection struct {
    Name    string
    Content string
}

type Config struct {
    SystemPrompt         string
    SystemPromptSections []SystemPromptSection
    // existing fields...
}
```

建议拼接顺序：

1. `SystemPrompt`
2. `SystemPromptSections`
3. SDK 自动追加的能力 section，例如 `Skills`

## Phase 1: Public SDK Section API

- [x] 在 `sdk` 包新增 `SystemPromptSection` 类型。
- [x] 在 `sdk.Config` 增加 `SystemPromptSections []SystemPromptSection` 字段。
- [x] 在 `sdk.AssembleConfig` 中将公开 section 转换为内部 prompt section。
- [x] 保持 section 为空或内容为空时自动跳过。
- [x] 添加测试覆盖 `SystemPrompt`、`SystemPromptSections`、skills section 的拼接顺序。

## Phase 2: Remove SDK-Owned Identity

- [x] 确认 `sdk.AssembleConfig` 不生成或注入默认 `Identity`。
- [x] 调整 `internal/systemprompt.Config`，让 identity 变成显式传入的字段。
- [x] 修改 `internal/systemprompt.Builder.Sections()`，仅在 identity 非空时输出 `# Identity`。
- [x] 更新测试，证明未传 identity 时不会出现默认 coding-agent 身份。
- [x] 保留 CLI 默认 identity，但移动到 `internal/cliconfig` 或 CLI 专属配置层传入。

## Phase 3: CLI Integration

- [x] 在 CLI prompt 构建时显式设置 coding-agent identity。
- [x] 确保 CLI 的 Environment、Git Snapshot、Project Instructions、Tool Guidance 等 section 顺序保持稳定。
- [x] 确认 CLI 禁用工具、启用 skills、启用 compaction 等行为不受影响。
- [x] 增加 CLI 配置测试，证明默认 CLI prompt 仍包含 CLI identity。
- [x] 增加测试，证明 CLI identity 与 SDK 默认行为解耦。

## Phase 4: Documentation and Examples

- [x] 更新 `docs/usage-sdk.md`，说明 SDK 不内置 identity，用户应自行定义。
- [x] 更新 `docs/design.md`，说明 system prompt 的职责边界：SDK 能力组装，应用定义身份。
- [x] 更新 `README.md` 中 SDK 快速示例，展示 `SystemPromptSections` 的最小用法。
- [x] 更新 `examples/sdk_walkthrough` 或新增示例，展示 Identity、Rules、Output Style section。
- [x] 更新 `AGENTS.md` 和 `CLAUDE.md`，记录新的 prompt/API 边界。
- [x] 在 `CHANGELOG.md` 的 `Unreleased` 中记录该 SDK prompt 扩展点。

## Phase 5: Validation

- [x] 运行 `go test ./sdk ./internal/systemprompt ./internal/cliconfig`。
- [x] 运行 `go test ./...`。
- [x] 运行 `make verify`。
- [x] 运行 `git diff --check`。
- [x] 手动检查默认 CLI prompt，确认 identity 只来自 CLI 层。

## Acceptance Criteria

- [x] SDK 用户可以用结构化 section 注入自定义 system prompt 内容。
- [x] SDK 不再默认注入 coding-agent identity。
- [x] CLI 默认 prompt 仍然包含明确的 coding-agent identity。
- [x] `SystemPrompt` 和 `SystemPromptSections` 的拼接顺序稳定且有测试保护。
- [x] Skills 等 SDK 自动能力说明仍然正常追加。
- [x] 文档清楚说明 identity 属于应用层，而不是通用 SDK 框架。

---

# System Prompt Architecture Refactor Plan

## Goal

将 Enno CLI 的 system prompt 从单个字符串拼接改造成可组合、可测试、可扩展的分段构建器。设计目标是吸收 Claude Code 的成熟经验，但保持 Enno 的轻量 SDK 定位：根包 `enno` 继续只负责 provider-neutral runtime，CLI 专属上下文和项目规则注入放在 `internal/*`，SDK 用户仍可完全自定义 `SystemPrompt`。

## Design Principles

- 保持根包 `enno` 不读取环境、git、配置文件或项目规则。
- SDK 继续透传用户提供的 `SystemPrompt`，只在高层 `sdk` 中追加明确的可选 section。
- CLI 使用默认 prompt builder；未来可通过配置禁用项目规则或环境注入。
- Prompt section 应有稳定名称、清晰职责和单元测试。
- 优先实现 Enno 当前有价值的能力，不照搬 Claude Code 的全部产品复杂度。
- 能用成熟 Go 库处理通用问题时优先使用库，只有现有库不能满足需求时才自行实现。

## Proposed Shape

```go
builder := systemprompt.New(systemprompt.Config{
    Identity: identity,
    SessionID: sessionID,
    Tools: systemprompt.ToolSet{
        TaskGraph: true,
        Filesystem: true,
        Shell: true,
        Grep: true,
        Glob: true,
        FetchURL: true,
        Subagent: false,
        Compact: true,
        LoadSkill: true,
    },
    ProjectInstructions: instructions,
    SkillsSummary: skillsSummary,
    CompactionEnabled: true,
})

prompt := builder.Build()
```

## Phase 1: Extract Current Prompt Builder

- [x] Create `internal/systemprompt` package for CLI-oriented prompt assembly.
- [x] Move current hard-coded CLI prompt text out of `internal/cliconfig.Parse`.
- [x] Represent prompt as ordered sections with `Name` and `Content`.
- [x] Add `Builder.Build() string` that joins non-empty sections with blank lines.
- [x] Add tests that preserve current CLI prompt content for default enabled tools.
- [x] Keep `internal/cliconfig` responsible for config parsing, not prose assembly.

## Phase 2: Tool Guidance Sections

- [x] Add `ToolSet` config to describe enabled built-in tools.
- [x] Move task graph guidance into a named `task_graph` section.
- [x] Move grep/glob/fetch_url/subagent/compact guidance into named sections.
- [x] Add missing filesystem and shell guidance:
  - [x] Prefer `read_file`, `write_file`, and `edit_file` over shell file operations.
  - [x] Prefer dedicated grep/glob tools over shell search commands.
  - [x] Reserve shell for terminal/system commands that require execution.
- [x] Document permission-denial behavior: if a tool is denied, adjust instead of retrying the same call.
- [x] Add tests for each enabled/disabled tool section.

## Phase 3: Environment Context

- [x] Add optional environment section for CLI runs.
- [x] Include primary working directory.
- [x] Include current local date.
- [x] Include platform and shell information using Go standard library APIs where sufficient.
- [x] Detect whether `workdir` is inside a git repository.
- [x] Keep expensive or failure-prone environment collection best-effort and non-fatal.
- [x] Add tests for deterministic formatting by injecting clock/env/git detector dependencies.

## Phase 4: Git Snapshot Context

- [x] Add a best-effort git snapshot section for CLI sessions.
- [x] Collect current branch, default branch when available, short status, recent commits, and git user.
- [x] Use `git --no-optional-locks` for status/log calls.
- [x] Truncate status output to a safe character limit.
- [x] State clearly that git status is a start-of-conversation snapshot.
- [x] Add config or internal option to disable git context if needed.
- [x] Add tests using a temporary git repository or mocked command runner.

## Phase 5: Project Instructions Loading

- [x] Add `internal/projectrules` or equivalent package for loading repository instructions.
- [x] Load `AGENTS.md` and `CLAUDE.md` from the working directory upward.
- [x] Preserve precedence: broader instructions first, closer directory instructions later.
- [x] Avoid duplicating identical files when both paths resolve to the same content.
- [x] Respect a max character budget per file and total project-instruction budget.
- [x] Wrap loaded content in a clear section explaining that project instructions override default prompt behavior.
- [x] Add tests for nested directories, missing files, truncation, and precedence.

## Phase 6: Skills Prompt Integration

- [x] Stop returning raw `systemPromptSuffix` strings from `sdk.buildChildTools`.
- [x] Introduce a structured skills summary result inside `sdk.AssembleConfig`.
- [x] Ensure `Skills available:` appears as a named section with stable boundaries.
- [x] Keep `load_skill` behavior unchanged.
- [x] Add tests proving SDK skill summaries still appear when skills are configured.

## Phase 7: Safety and Task Behavior Guidance

- [x] Add concise default guidance for reading code before modifying it.
- [x] Add guidance to avoid unrelated refactors and speculative abstractions.
- [x] Add guidance to verify changes and report verification honestly.
- [x] Add guidance about external tool-result prompt injection risks.
- [x] Keep this section short enough that CLI prompt remains practical for small models.
- [x] Add golden tests for the default prompt shape.

## Phase 8: CLI Integration

- [x] Replace inline `sys := fmt.Sprintf(...)` in `internal/cliconfig.Parse` with `internal/systemprompt`.
- [x] Pass enabled tool state, session ID, compaction state, workdir, and loaded project instructions to the builder.
- [x] Keep existing CLI YAML flags and defaults unchanged.
- [x] Add tests for default CLI prompt, disabled tools, disabled task graph, and compaction text.
- [x] Ensure `CLAUDE_CODE_DISABLE_MOUSE` behavior remains unrelated to prompt assembly.

## Phase 9: Documentation Updates

- [x] Update `docs/design.md` with the new prompt section architecture.
- [x] Update `docs/usage-cli.md` to describe project instruction loading and environment/git context.
- [x] Update `docs/usage-sdk.md` to clarify SDK `SystemPrompt` behavior versus CLI defaults.
- [x] Update `README.md` only with concise user-facing notes.
- [x] Update `AGENTS.md` and `CLAUDE.md` when dependency direction or prompt responsibilities change.
- [x] Add `CHANGELOG.md` entry under `Unreleased`.

## Phase 10: Validation

- [x] Run focused tests for `internal/systemprompt`, `internal/projectrules`, `sdk`, and `internal/cliconfig`.
- [x] Run `go test ./...`.
- [x] Run `make verify`.
- [x] Run `git diff --check`.
- [x] Manually inspect the generated default CLI prompt for readability and duplication.

## Acceptance Criteria

- [x] CLI system prompt is assembled from named, tested sections.
- [x] Existing CLI tool enable/disable behavior is preserved.
- [x] SDK users remain in control of their own `SystemPrompt`.
- [x] Project `AGENTS.md`/`CLAUDE.md` instructions can be injected into CLI prompts with clear precedence.
- [x] Environment and git context are useful but best-effort and non-fatal.
- [x] Prompt text is easier to evolve without editing `internal/cliconfig.Parse`.
- [x] Documentation accurately describes the new architecture and behavior.
