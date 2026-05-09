# CLI Extraction Boundary Cleanup Plan

## Goal

为后续将 CLI 拆成独立项目做边界清理。SDK 仓库应只保留 provider-neutral
core、provider adapters、高层 SDK assembler 和可复用 built-in tools；CLI
项目应拥有自己的配置解析、TUI/REPL、history、coding-agent prompt、项目规则加载、
默认目录、安装与发布流程。

## Design Principles

- 依赖方向必须保持单向：CLI 应用依赖 `github.com/dean2021/enno`、`sdk` 和 `provider/*`，SDK 不能依赖 CLI 包。
- 根包 `enno` 不应读取环境、用户 home、配置文件、git 状态、项目规则，或选择 CLI 品牌目录。
- provider 包只负责 provider SDK 适配和 HTTP 传输配置，不依赖 CLI helper。
- CLI prompt、project rules、history、TUI、YAML flags/config、默认路径均属于 CLI 应用层。
- 拆分前先消除语义不清的包归属，避免未来移动代码时破坏 provider 或 SDK。
- 文档、Makefile、release docs 必须和最终边界一致。

## Phase 1: Package Ownership Audit

- [x] 建立包归属表，明确哪些包留在 SDK 仓库、哪些未来迁入 CLI 仓库。
- [x] 将 `cmd/enno`、`internal/cliconfig`、`internal/cliui`、`internal/history`、`internal/cliprompt`、`internal/projectrules` 标记为 CLI-owned。
- [x] 将 `enno` 根包、`sdk`、`provider/*`、`builtintools/*`、`prompt` 标记为 SDK-owned。
- [x] 检查 SDK-owned 包是否导入 CLI-owned 包；发现即修复。
- [x] 检查 CLI-owned 包是否只通过公开 API 使用 SDK/provider，不访问 SDK internal 细节。
- [x] 添加或更新文档中的依赖方向图。

## Phase 2: HTTP Proxy Ownership Cleanup

- [x] 重新归属 `internal/httpproxy`：它当前被 `provider/openai` 和 `provider/anthropic` 使用，不应继续描述为 CLI-only。
- [x] 将 `internal/httpproxy` 移到 provider-owned 位置，例如 `provider/internal/httpproxy`，或重命名为 SDK-owned `internal/httptransport`。
- [x] 更新 `provider/openai` 和 `provider/anthropic` imports。
- [x] 更新相关测试路径和包名。
- [x] 更新 `README.md`、`docs/design.md`、`AGENTS.md`、`CLAUDE.md` 中关于 `httpproxy` 的描述。
- [x] 验证 CLI 拆出后 provider 包仍能独立构建。

## Phase 3: Remove CLI-Branded Defaults From Core

- [x] 调整根包 `CompactionConfig` 默认行为，避免在 SDK core 中默认写入 `~/.enno/transcripts`。
- [x] 将 CLI 默认 transcript dir 移到 `internal/cliconfig`，由 CLI 显式设置。
- [x] 确认 SDK 用户启用 compaction 但未设置 `TranscriptDir` 时的行为清晰、可测试、无 CLI 品牌路径。
- [x] 更新 compaction 相关测试，覆盖 SDK 默认和 CLI 默认两种行为。
- [x] 更新 `docs/usage-sdk.md` 与 `docs/usage-cli.md`，区分 SDK 配置和 CLI 默认。

## Phase 4: Simplify CLI Prompt Boundary

- [x] 移除 `internal/cliprompt.CodingAgentConfig.SkillsSummary` 和 `cliprompt` 内部 skills section。
- [x] 保持 skills runtime section 只由 SDK `prompt.RuntimeSections` 追加。
- [x] 更新 `internal/cliprompt` 测试，确保 CLI prompt builder 不再拥有 SDK runtime capability section。
- [x] 更新 `internal/cliconfig` 测试，确认启用 `load_skill` 时 skills 仍由 SDK prompt assembly 注入。
- [x] 更新文档中关于 CLI prompt 与 SDK runtime sections 的职责说明。

## Phase 5: Decouple CLI UI From Concrete History

- [x] 将 `internal/cliui.Config.Recorder *history.Recorder` 改为小接口，例如 `interface { Record(string) error }`。
- [x] 让 `internal/history.Recorder` 继续实现该接口。
- [x] 调整 `cmd/enno` 注入 recorder 的方式。
- [x] 更新 `internal/cliui` 测试，避免 UI 测试必须构造具体 history recorder。
- [x] 保持 CLI history 存储仍属于 CLI 应用层，不进入 SDK。

## Phase 6: Prepare Module Split

- [x] 梳理 `go.mod` 依赖，标记哪些依赖只由 CLI 使用：Bubble Tea、Bubbles、Lipgloss、YAML 等。
- [x] 确认 SDK-owned 包不依赖 CLI-only 依赖。
- [x] 设计未来 CLI 仓库的 module path 和导入形态，例如 CLI module 依赖 `github.com/dean2021/enno`。
- [x] 列出 CLI 仓库需要迁移的目录：`cmd/enno`、`internal/cliconfig`、`internal/cliui`、`internal/history`、`internal/cliprompt`、`internal/projectrules`。
- [x] 确认迁移后 CLI 不能引用 SDK 仓库的 `internal/*` 包；需要的能力必须是公开 API 或迁移到 CLI 仓库。
- [x] 检查 examples/docs 是否仍应该留在 SDK 仓库，或将 CLI 文档迁移到 CLI 仓库。

## Phase 7: Release and Tooling Separation

- [x] 调整 SDK 仓库 `Makefile`，让 `make verify` 不再强制 `go install ./cmd/enno`。
- [x] 增加或保留临时 CLI verify 目标，直到 CLI 真正拆仓。
- [x] 更新 `docs/release.md`，区分 SDK release 和 CLI release。
- [x] 更新 `README.md` 安装说明，标注 CLI 未来会迁移到独立仓库。
- [x] 检查 `CHANGELOG.md` 记录方式，决定 SDK 与 CLI 是否分开维护 changelog。

## Phase 8: Documentation and Contributor Rules

- [x] 更新 `AGENTS.md` 和 `CLAUDE.md`，明确 CLI-owned 包的代码只能依赖 SDK 公开 API。
- [x] 更新 `docs/design.md` 的架构图和数据流图，展示未来拆分边界。
- [x] 更新 `docs/usage-sdk.md`，移除会让 SDK 用户误解 CLI 默认行为的描述。
- [x] 更新 `docs/usage-cli.md`，标注哪些行为属于 CLI 应用层默认。
- [x] 检查 `README.md` 项目结构，避免把 provider/shared helper 错写成 CLI helper。

## Phase 9: Validation

- [x] 运行 `go test ./...`。
- [x] 运行 `make verify`。
- [x] 运行 `git diff --check`。
- [x] 运行包依赖检查，确认 `sdk` 不导入任何 CLI-owned 包。
- [x] 运行包依赖检查，确认 `provider/*` 不导入 CLI-owned 包。
- [x] 手动检查文档中的 dependency graph 与实际 imports 一致。

## Acceptance Criteria

- [x] SDK-owned 包没有 CLI-owned imports。
- [x] Provider proxy helper 不再被描述或组织为 CLI-only。
- [x] 根包不再默认使用 `~/.enno` 这类 CLI 品牌路径。
- [x] CLI prompt builder 不再重复 SDK runtime skills section。
- [x] CLI UI 不依赖具体 history recorder 类型。
- [x] SDK verify/release 流程不再强绑定 CLI install。
- [x] 文档清楚说明 CLI 未来可作为独立项目依赖 Enno SDK。

---

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
- 公开 API 不暴露 `prompt.Section` 或 `internal/cliprompt.Section`，避免把内部实现泄漏给 SDK 用户。
- 保留简单字符串入口，同时增加结构化 section 入口；用户可按复杂度选择。
- SDK 自动追加的内容应只描述通用运行时能力，例如 skills 摘要，不覆盖用户 identity。
- 工具使用建议应放在对应 `enno.Tool.Description` 中，不再放入全局 `Tool Guidance` system prompt section。

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
- [x] 调整 CLI prompt builder 配置，让 identity 变成显式传入的字段。
- [x] 修改 CLI prompt builder 的 sections，仅在 identity 非空时输出 `# Identity`。
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

- [x] 运行 `go test ./sdk ./prompt ./internal/cliprompt ./internal/cliconfig`。
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
builder := cliprompt.NewCodingAgent(cliprompt.CodingAgentConfig{
    Identity: identity,
    ProjectInstructions: instructions,
    SkillsSummary: skillsSummary,
    CompactionEnabled: true,
})

prompt := builder.Build()
```

## Phase 1: Extract Current Prompt Builder

- [x] Create `internal/cliprompt` package for CLI-oriented prompt assembly.
- [x] Move current hard-coded CLI prompt text out of `internal/cliconfig.Parse`.
- [x] Represent prompt as ordered sections with `Name` and `Content`.
- [x] Add `Builder.Build() string` that joins non-empty sections with blank lines.
- [x] Add tests that preserve current CLI prompt content for default enabled tools.
- [x] Keep `internal/cliconfig` responsible for config parsing, not prose assembly.

## Phase 2: Tool Guidance Sections

- [x] Move tool-specific guidance into tool descriptions instead of prompt `ToolSet` sections.
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

- [x] Replace inline `sys := fmt.Sprintf(...)` in `internal/cliconfig.Parse` with `internal/cliprompt`.
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

- [x] Run focused tests for `prompt`, `internal/cliprompt`, `internal/projectrules`, `sdk`, and `internal/cliconfig`.
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

---

# Godo CLI Migration Plan

## Goal

将当前仓库中的 CLI 从 `enno` 拆分到独立目录
`../godo-coding-agent`，新 CLI
项目名为 `godo`。拆分后：Enno 仓库聚焦 SDK/provider，Godo 仓库聚焦 CLI 应用。

## Phase 1: Scope and Baseline

- [x] 明确迁移边界：仅迁移 CLI-owned 代码（`cmd/enno`、`internal/cliconfig`、`internal/cliui`、`internal/history`、`internal/cliprompt`、`internal/projectrules`）。
- [x] 记录当前 CLI 可执行行为基线（命令、配置加载、会话、工具开关、TUI 流程）。
- [x] 确认目标目录存在并初始化为独立 git/go module 工程。

## Phase 2: Bootstrap New Godo Module

- [x] 在目标目录初始化 `go.mod`（module 名以 `godo` 项目命名）。
- [x] 建立基础工程骨架：`cmd/godo`、`internal/*`、`docs`、`Makefile`、`VERSION`、`CHANGELOG.md`。
- [x] 配置依赖：通过公开 API 引入 `github.com/dean2021/enno`、`sdk`、`provider/openai`、`provider/anthropic`。

## Phase 3: Move and Rename CLI Code

- [x] 迁移 CLI-owned 目录到 Godo 仓库，并修正 import path。
- [x] 将命令入口从 `cmd/enno` 重命名为 `cmd/godo`，二进制名改为 `godo`。
- [x] 统一用户可见文案中的品牌名：`enno` -> `godo`（提示符、帮助信息、错误提示、示例命令）。

## Phase 4: Runtime Paths and Config

- [x] 将 CLI 默认配置目录、history、tasks、skills、transcripts 路径迁移到 `~/.godo/*`。
- [x] 明确是否提供一次性迁移逻辑（从 `~/.enno` 导入配置/历史）；若不提供，在文档中给出手动迁移步骤。
- [x] 保持 SDK core 无 CLI 路径默认值，全部 CLI 默认行为只存在于 Godo 仓库。

## Phase 5: Decoupling Verification

- [x] 验证 Godo 不引用 Enno 仓库 `internal/*` 包。
- [x] 验证 Enno 仓库移除 CLI 后仍可 `go test ./...` 和 `make verify` 通过。
- [x] 为 Godo 增加最小验证链路：`make fmt`、`make test`、`make install`、`make verify`。

## Phase 6: Documentation and Release

- [x] 在 Enno 仓库更新 README/docs：CLI 已迁移到 `godo-coding-agent`，SDK 使用路径不变。
- [x] 在 Godo 仓库补齐 README、配置说明、CLI 使用文档、发布流程。
- [x] 更新两个仓库的 `AGENTS.md`/`CLAUDE.md`，明确边界和依赖方向。
- [x] 记录迁移发布说明：Enno 发布去 CLI 化版本，Godo 发布首个独立版本。

## Acceptance Criteria

- [x] `godo` 可独立构建、安装、运行，并覆盖现有 CLI 关键功能。
- [x] Enno 仓库不再包含 CLI 入口与 CLI-owned internal 包。
- [x] 两个仓库的文档、命令、模块路径、发布说明一致且可执行。
