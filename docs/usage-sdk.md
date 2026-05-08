# Enno SDK 使用指南

本文档介绍如何将 Enno 作为 Go Package 嵌入到自己的项目中。CLI 使用请参考 [CLI 使用指南](usage-cli.md)。

## 安装

```sh
go get github.com/dean2021/enno@latest
```

## 基础用法

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

    provider, err := openaiprovider.New(openaiprovider.Config{
        APIKey:  os.Getenv("ENNO_API_KEY"),
        BaseURL: os.Getenv("ENNO_BASE_URL"),
        Model:   os.Getenv("ENNO_MODEL"),
    })
    if err != nil {
        panic(err)
    }

    agent, err := enno.NewAgent(enno.Config{
        Provider:     provider,
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

## Provider

### OpenAI 兼容 Provider

```go
provider, err := openaiprovider.New(openaiprovider.Config{
    APIKey:  "your-key",
    BaseURL: "https://example.com/v1",
    Model:   "your-model",
})
```

可选配置：

- `MaxHTTPRetries`：SDK HTTP 重试次数（429 / 5xx / 超时 / 连接错误），默认 6。
- `HTTPProxy`：HTTP 或 SOCKS5 代理 URL，例如 `"http://127.0.0.1:7890"` 或 `"socks5://127.0.0.1:7891"`。为空则走 SDK 默认客户端。

### Anthropic Provider

```go
import anthropicprovider "github.com/dean2021/enno/provider/anthropic"

provider, err := anthropicprovider.New(anthropicprovider.Config{
    APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
    Model:     "claude-sonnet-4-5-20250929",
    MaxTokens: 4096,
})
if err != nil {
    panic(err)
}
agent, err := enno.NewAgent(enno.Config{
    Provider:     provider,
    SystemPrompt: "You are a helpful agent.",
    Tools:        taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second}),
})
```

可选配置同 OpenAI：`MaxHTTPRetries` 和 `HTTPProxy`。

### 自定义 Provider

实现 `enno.Provider` 接口即可接入任意模型后端：

```go
type MyProvider struct{}

func (p *MyProvider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
    // 将 req 转换为你的模型 API 调用，然后返回 enno.Response
}
```

## 自定义工具

### NewTypedTool（推荐）

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

### NewTool（原始 JSON）

```go
tool := enno.NewTool("my_tool", "Description", properties, required,
    func(ctx context.Context, raw json.RawMessage) (string, error) {
        // 手动解析 raw
        return "result", nil
    },
)
```

将工具传给 Agent：

```go
agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    []enno.Tool{greet},
})
```

## 内置工具

### 任务图

使用 `task_create` / `task_update` / `task_list` / `task_get` 维护持久化 DAG。若 `taskgraph.Config.TasksDir` 为空，默认使用 `Root/.tasks/`。若注册了任一 `task_*` 工具，Agent 在连续多轮只执行其它工具而未使用任务图工具时，可注入 `<reminder>Update your task plan.</reminder>`（见 [design.md](design.md)）。

```go
import "github.com/dean2021/enno/tools/taskgraph"

tools := taskgraph.New(taskgraph.Config{Root: ".", Timeout: 120 * time.Second})
```

### 文件系统

```go
import "github.com/dean2021/enno/tools/filesystem"

tools = append(tools, filesystem.New(filesystem.Config{
    Root: ".",
})...)
```

提供 `read_file`、`write_file`、`edit_file`，通过 `Config.Root` 限制文件访问范围。

### Shell

```go
import "github.com/dean2021/enno/tools/shell"

tools = append(tools, shell.New(shell.Config{
    Workdir:  ".",
    Timeout:  120 * time.Second,
    DenyList: nil, // 使用默认禁止列表
}))
```

提供 `bash` 工具，受 `Workdir`、`Timeout` 和 `DenyList` 约束。

### Grep（ripgrep 内容搜索）

```go
import "github.com/dean2021/enno/tools/grep"

tools = append(tools, grep.New(grep.Config{
    Root:    ".",
    Timeout: 120 * time.Second,
}))
```

需要系统已安装 `rg`。

### Glob（文件名匹配）

```go
import "github.com/dean2021/enno/tools/glob"

tools = append(tools, glob.New(glob.Config{
    Root:    ".",
    Timeout: 120 * time.Second,
}))
```

同样依赖系统 `rg`。

### Subagent

先组装子工具列表（不含 `subagent`），再创建 subagent 工具并挂到父 Agent：

```go
import "github.com/dean2021/enno/tools/subagent"

childTools := []enno.Tool{ /* taskgraph.New(...), filesystem, shell, ... */ }
subagentTool, err := subagent.New(subagent.Config{
    Provider:      provider,
    ChildTools:    childTools,
    SystemPrompt:  "",                             // 空 = 使用子工具默认提示
    MaxToolRounds: 0,                              // 0 = 不限制
    MaxResultChars: 50000,                         // 结果截断字节
})
if err != nil {
    return err
}
parentTools := append(append([]enno.Tool{}, childTools...), subagentTool)
```

子 Agent 使用空历史、独立 system prompt，运行结束后仅将最终文本回复写回父对话。子工具列表中不得再包含 `subagent`。

### Skills（load_skill）

```go
import "github.com/dean2021/enno/tools/loadskill"

reg, err := loadskill.LoadDirs([]string{os.Getenv("HOME") + "/.enno/skills", "/opt/my-skills"})
if err != nil {
    return err
}
if reg.Count() == 0 {
    // 没有 skill
}
loadTool, err := loadskill.NewTool(reg)
if err != nil {
    return err
}
tools = append(tools, loadTool)
```

`LoadDirs` 按顺序合并目录，后者覆盖同名 skill。使用时在 system prompt 尾部拼接技能摘要：

```go
systemPrompt := "You are a helpful agent.\n\nSkills available:\n" + reg.DescriptionsText()
```

### Compact（上下文压缩）

在装配 `tools/compact` 的同时设置 `Compaction`（默认 `nil` 为关闭）：

```go
import compacttool "github.com/dean2021/enno/tools/compact"

agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    append(tools, compacttool.New()),
    Compaction: &enno.CompactionConfig{
        Enabled: true,
        // ModelContextTokens: 200000,       // 与 AutoCompactBufferTokens 联合决定阈值；否则用 AutoCompactInputTokens
        // AutoCompactInputTokens: 50000,    // 默认 50000
        // AutoCompactBufferTokens: 13000,    // 默认 13000；仅 ModelContextTokens > 0 时生效
        // KeepRecentToolResults: 3,          // micro 保留最近 N 条工具结果
        // MicroCompactMinChars: 100,         // short tool results stay full
        // MicroCompactToolNames: []string{"bash", "read_file"}, // 仅对这些工具名触发 micro
        // SkipOnSummarizeError: true,        // 自动摘要失败时跳过而非中断
        // TranscriptDir: "",                 // 空 = withDefaults 使用 ~/.enno/transcripts
    },
})
```

`CompactionConfig.withDefaults()` 会填充零值为合理默认值；库用法不配置 `Compaction`（即 `nil`）时压缩完全关闭。

## 执行 Agent

SDK 用户直接调用 `Agent.Run`：

```go
answer, err := agent.Run(ctx, "总结当前项目")
```

单次 `Run` 内 `MaxToolRounds` 默认为 0（不限制工具/模型轮数）；需要硬上限时在 `enno.Config` 中设置正整数。

`Agent` 内部持有互斥锁，同一实例的 `Run` 调用串行执行。并发会话请创建多个 `Agent` 实例。

## 观察 Agent 事件

通过 `EventHandler` 观察模型调用、工具调用、工具结果和 token usage：

```go
agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    tools,
    EventHandler: func(ctx context.Context, event enno.Event) {
        fmt.Printf("%s round=%d usage=%+v\n", event.Type, event.Round, event.Usage)
    },
})
```

事件只包含可观测执行过程和模型显式返回内容，不包含隐藏思维链。

事件类型：

- `EventModelStart`：开始调用模型
- `EventModelResponse`：模型返回
- `EventToolStart`：开始执行工具
- `EventToolResult`：工具执行结果
- `EventRoundComplete`：一轮结束
- `EventError`：错误

## 包说明

| 包 | 说明 |
|---|---|
| `github.com/dean2021/enno` | 核心 Agent API |
| `github.com/dean2021/enno/provider/openai` | OpenAI Chat Completions 兼容 provider |
| `github.com/dean2021/enno/provider/anthropic` | Anthropic Messages API provider |
| `github.com/dean2021/enno/tools/taskgraph` | 持久化任务图工具（`task_*`） |
| `github.com/deanlu/enno/tools/filesystem` | 受根目录限制的文件读写编辑工具 |
| `github.com/dean2021/enno/tools/shell` | 受工作目录和 denylist 限制的 shell 工具 |
| `github.com/dean2021/enno/tools/grep` | ripgrep 内容搜索工具 |
| `github.com/dean2021/enno/tools/glob` | ripgrep 文件名 glob 工具 |
| `github.com/dean2021/enno/tools/subagent` | 子 Agent 委派工具 |
| `github.com/dean2021/enno/tools/loadskill` | SKILL.md 加载与检索工具 |
| `github.com/dean2021/enno/tools/compact` | 上下文压缩触发工具 |

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