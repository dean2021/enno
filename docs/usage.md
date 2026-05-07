# Enno Usage Guide

## 安装 CLI

本地开发时可以在项目根目录安装：

```sh
go install ./cmd/enno
```

项目发布到 GitHub 后，用户可以通过模块路径安装：

```sh
go install github.com/dean2021/enno/cmd/enno@latest
```

安装后运行：

```sh
enno
```

## CLI 使用

### 交互模式

```sh
enno
```

进入基于 `tview` 的 TUI 交互界面后输入任务并按 Enter 提交。可以使用 `Esc`、`Ctrl+C`、`q` 或 `exit` 退出。主 `Enno` 窗口是单一会话流：用户输入、模型进度、工具调用、工具参数、弱化显示的工具结果和最终回答会按时间顺序追加，便于直观看到当前正在进行的动作。主会话区**仅**支持在主窗口上方用 **鼠标滚轮** 滚动（键盘不再滚动主窗口）。底部输入框用 **↑ / ↓** 浏览历史输入。启用鼠标协议后，部分终端里划选需 **Shift+拖拽**；**F9** 可将当前会话纯文本复制到剪贴板。它不会展示模型隐藏思维链。

### 单次执行

```sh
enno run "帮我分析当前目录的 Go 包结构"
```

### 配置文件

CLI 会自动尝试读取：

```text
~/.enno/config.yaml
```

默认配置文件不存在时，CLI 会自动创建一个带注释的模板文件，然后继续按现有配置解析。Provider、model、api key、base URL、max tokens 等模型配置只从 YAML 读取，不再读取 `ENNO_*` 环境变量，也不再提供对应 flags。也可以显式指定配置文件：

```sh
enno --config /path/to/config.yaml
enno run --config /path/to/config.yaml "帮我分析当前目录"
```

OpenAI 兼容配置示例：

```yaml
provider: openai
model: your-model
api_key: your-key
base_url: https://example.com/v1
max_tokens: 4096
shell: true
filesystem: true
```

Anthropic 配置示例：

```yaml
provider: anthropic
model: claude-sonnet-4-5-20250929
api_key: your-anthropic-key
max_tokens: 4096
shell: false
filesystem: true
```

字段说明：

- `provider`：模型供应商，支持 `openai` 或 `anthropic`。
- `model`：模型名称，所有 provider 必填。
- `api_key`：供应商 API key。
- `base_url`：OpenAI 兼容 provider 必填。
- `max_tokens`：Anthropic 最大输出 token 数。
- `shell`：设为 `false` 等价于 `--no-shell`。
- `filesystem`：设为 `false` 等价于 `--no-filesystem`。
- `subagent`：设为 `true` 时在父 Agent 上额外注册 `task` 工具（独立上下文的子 Agent）；默认为关闭，避免额外模型调用。等价于命令行不显式传 `--no-subagent` 且配置开启。
- `grep`：设为 `false` 等价于 `--no-grep`，关闭 **`grep`**（ripgrep 内容搜索）工具；**默认开启**（省略时启用）。需要系统安装 [`ripgrep`](https://github.com/BurntSushi/ripgrep)（`rg` 在 `PATH` 中）。
- `glob`：设为 `false` 等价于 `--no-glob`，关闭 **`glob`**（`rg --files` 按文件名 glob）工具；**默认开启**（省略时启用）。同样依赖系统 **`rg`**。默认每个请求最多返回约 **100** 条路径（可在工具参数中调整 `limit`；`0` 表示不截断，可能输出很大）。
- `task_graph`：设为 `false` 等价于 `--no-task-graph`，关闭四个 **`task_*`** 任务图工具；**默认开启**（省略时启用）。**CLI** 下任务数据写入 **`~/.enno/tasks/<session_id>/`**（`session_id` 为本次进程 UUID v4，**不**随 `--workdir` 变化；`grep`/`bash` 等仍受 `--workdir` 约束）。
- 技能目录（合并使用，**后者覆盖同名 skill**）：
  - 默认始终包含 **`~/.enno/skills`**（若该路径不存在则跳过，不报错）。
  - `skills_extra_dirs`：字符串列表，每项为含 `**/SKILL.md` 的根目录；支持 `~`。
  - `skills_dir`（可选）：单个额外目录，与列表并存时排在 `skills_extra_dirs` 之后合并。
  - 任意路径存在且可读但不是目录时，`Parse` 会报错；**缺失的目录会跳过**。
  - 合并后若至少解析到一个 skill，则注册 `load_skill` 并在 system prompt 中追加摘要。
- `compaction`：上下文压缩。**CLI 自动创建的模板文件默认带有 `compaction.enabled: true`**（可按需改为 `false` 关闭）；库用法仍为不显式配置则不启用。启用后会注册 `compact`、执行 micro；**仅**在估算超过阈值或模型调用 `compact` 时才会多一次摘要模型调用（计费），并在 `transcript_dir`（默认 `~/.enno/transcripts`）写入 JSONL（可用 `transcript_dir` 覆盖，支持 `~`）。
  - `compaction: true`：等价于启用默认阈值等并注册 `compact`。
  - 或映射形式，例如：
    ```yaml
    compaction:
      enabled: true
      transcript_dir: ~/.enno/transcripts
      model_context_tokens: 200000
      auto_compact_buffer_tokens: 13000
      auto_compact_input_tokens: 50000
      keep_recent_tool_results: 3
      micro_compact_min_chars: 100
      micro_compact_tool_names:
        - bash
        - read_file
      skip_on_summarize_error: false
    ```
  - `model_context_tokens`：若设置，自动压缩阈值优先为「该值 − `auto_compact_buffer_tokens`」（buffer 默认 13000）；否则使用 `auto_compact_input_tokens`。
  - `skip_on_summarize_error`：摘要 API 失败时是否跳过替换历史（仅影响**自动**压缩；手动 `compact` 仍报错）。
  - 映射形式须设置 `enabled: true` 才会开启；仅注册 `compact` 工具并在启用时附加说明到 system prompt。

### 常用 Flags

```sh
enno --workdir .
enno --no-shell
enno --no-filesystem
enno --no-grep
enno --no-glob
enno --no-task-graph
enno --no-subagent
enno --skills-dir /path/to/more-skills
```

`--no-grep` 可关闭 `grep` 工具（与配置中的 `grep: true` 组合时，以关闭为准）。`--no-glob` 可关闭 `glob` 工具。`--no-task-graph` 可关闭任务图四工具。`--no-subagent` 可关闭子代理 `task` 工具（与配置中的 `subagent: true` 组合时，以关闭为准）。`--skills-dir` 在合并顺序上排在默认目录与 YAML 配置之后，用于临时追加一个扩展目录（路径会去重）。

CLI 默认在子 Agent 与父 Agent 中装配同一套**子工具**（顺序为）：**`task_*`**（若开启）→ 文件系统工具（若开启）→ `bash`（若开启）→ **`grep`**（若开启）→ **`glob`**（若开启）→ 其他（如 `load_skill` 等）→ 需要时 `compact` → 若开启子代理则 **`task`**（子 Agent）。

## 作为 Go Package 使用

### 基础用法

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/dean2021/enno"
    openaiprovider "github.com/dean2021/enno/provider/openai"
    "github.com/dean2021/enno/tools/filesystem"
    "github.com/dean2021/enno/tools/taskgraph"
)

func main() {
    tools := taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second})
    tools = append(tools, filesystem.New(filesystem.Config{Root: "."})...)

    agent, err := enno.NewAgent(enno.Config{
        Provider: openaiprovider.New(openaiprovider.Config{
            APIKey:  os.Getenv("ENNO_API_KEY"),
            BaseURL: os.Getenv("ENNO_BASE_URL"),
            Model:   os.Getenv("ENNO_MODEL"),
        }),
        SystemPrompt: "You are a helpful coding agent.",
        Tools:        tools,
    })
    if err != nil {
        panic(err)
    }

    answer, err := agent.Run(context.Background(), "查看当前目录有哪些文件")
    if err != nil {
        panic(err)
    }
    fmt.Println(answer)
}
```

### 使用 Anthropic

```go
agent, err := enno.NewAgent(enno.Config{
    Provider: anthropicprovider.New(anthropicprovider.Config{
        APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
        Model:     "claude-sonnet-4-5-20250929",
        MaxTokens: 4096,
    }),
    SystemPrompt: "You are a helpful agent.",
    Tools:        taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second}),
})
```

需要导入：

```go
import anthropicprovider "github.com/dean2021/enno/provider/anthropic"
```

### 自定义工具

```go
type GreetArgs struct {
    Name string `json:"name"`
}

greet := enno.NewTypedTool("greet", "Greet a person by name.", map[string]any{
    "name": map[string]any{"type": "string"},
}, []string{"name"}, func(ctx context.Context, args GreetArgs) (string, error) {
    return "Hello, " + args.Name + "!", nil
})
```

将工具传给 Agent：

```go
agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    []enno.Tool{greet},
})
```

### 上下文压缩（库配置）

在装配 `tools/compact` 的同时设置 `Compaction`（默认 `nil` 为关闭）：

```go
import compacttool "github.com/dean2021/enno/tools/compact"

agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    append(tools, compacttool.New()),
    Compaction: &enno.CompactionConfig{
        Enabled: true,
        // ModelContextTokens: 200000, // 与 AutoCompactBufferTokens 联合决定阈值；否则用 AutoCompactInputTokens
        // MicroCompactToolNames: []string{"bash"},
        // SkipOnSummarizeError: true,
        // TranscriptDir 为空且 Enabled 时，withDefaults 会使用 ~/.enno/transcripts
    },
})
```

### 使用内置工具

任务图（`tools/taskgraph`）：使用 **`task_create` / `task_update` / `task_list` / `task_get`** 维护持久化 DAG。在 **Go 中自行组装**时，若 `taskgraph.Config.TasksDir` 为空，默认使用 **`Root/.tasks/`**；**CLI** 则使用 **`~/.enno/tasks/<session_id>/`**。若注册了任一 **`task_*`** 工具，根包在连续多轮只执行其它工具而未使用任务图工具时，可注入 `<reminder>Update your task plan.</reminder>`（见 `docs/design.md`）。

```go
import "time"
import "github.com/dean2021/enno/tools/taskgraph"

tools := taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second})
```

文件工具：

```go
tools = append(tools, filesystem.New(filesystem.Config{
    Root: ".",
})...)
```

Shell 工具：

```go
tools = append(tools, shell.New(shell.Config{
	Workdir: ".",
	Timeout: 120 * time.Second,
}))
```

Subagent（`task`）工具：先组装子工具列表（不含 `task`），再生成 `task` 并挂到父 Agent 上。

```go
import "github.com/dean2021/enno/tools/subagent"

childTools := []enno.Tool{ /* taskgraph.New(...), filesystem, shell, ... */ }
taskTool, err := subagent.New(subagent.Config{
	Provider:   provider,
	ChildTools: childTools,
})
if err != nil {
	return err
}
parentTools := append(append([]enno.Tool{}, childTools...), taskTool)
```

Skills（`load_skill`）：在 Go 中可用 `loadskill.LoadDir` 扫描单根目录，或用 `loadskill.LoadDirs` 按顺序合并多根目录（后者覆盖同名 skill）。将 `loadskill.NewTool(reg)` 加入 `Tools`，并在 system prompt 中拼接 `Skills available:\n` + `reg.DescriptionsText()`。

```go
import "github.com/dean2021/enno/tools/loadskill"

reg, err := loadskill.LoadDirs([]string{os.Getenv("HOME") + "/.enno/skills", "/opt/my-skills"})
if err != nil {
	return err
}
if reg.Count() == 0 {
	// no skills
}
loadTool, err := loadskill.NewTool(reg)
if err != nil {
	return err
}
tools = append(tools, loadTool)
```

### 执行 Agent

SDK 用户直接调用 `Agent.Run`。REPL/TUI 属于 `cmd/enno` 的 CLI 表现层，不作为公共 Go package 暴露：

```go
answer, err := agent.Run(ctx, "总结当前项目")
```

### 观察 Agent 事件

SDK 用户可以通过 `EventHandler` 观察模型调用、工具调用、工具结果和 token usage：

```go
agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    tools,
    EventHandler: func(ctx context.Context, event enno.Event) {
        fmt.Printf("%s round=%d usage=%+v\n", event.Type, event.Round, event.Usage)
    },
})
```

事件只包含可观测执行过程和模型显式返回内容，不包含隐藏思维链。CLI 会把这些事件直接追加到主 `Enno` 会话流中：模型进度、工具调用、参数和弱化显示的结果会像 coding agent transcript 一样按发生顺序出现。

## 包说明

- `github.com/dean2021/enno`：核心 Agent API。
- `github.com/dean2021/enno/provider/openai`：OpenAI Chat Completions 兼容 provider。
- `github.com/dean2021/enno/provider/anthropic`：Anthropic Messages API provider。
- `github.com/dean2021/enno/tools/taskgraph`：持久化任务图工具（`task_*`）。
- `github.com/dean2021/enno/tools/filesystem`：受根目录限制的文件读写编辑工具。
- `github.com/dean2021/enno/tools/shell`：受工作目录和 denylist 限制的 shell 工具。
- `github.com/dean2021/enno/cmd/enno`：可安装 CLI。

## 安全建议

- 生产环境不要默认开启 `tools/shell`，除非运行环境有隔离措施。
- `tools/filesystem` 必须指定合适的 `Root`，避免模型访问不该访问的路径。
- API key 应通过环境变量或密钥管理系统注入，不要写入代码。
- 每个用户会话建议使用独立 `Agent` 实例，避免共享历史和工具状态。

## 示例

仓库内置示例：

```sh
go run ./examples/simple_agent
go run ./examples/custom_tool
go run ./examples/anthropic
```

这些示例分别展示基础 Agent、自定义工具和 Anthropic provider 的用法。
