# Enno SDK 使用指南

本文档介绍如何将 Enno 作为 Go Package 嵌入到自己的项目中。CLI 使用请参考 [CLI 使用指南](usage-cli.md)。

## SDK 稳定性

Enno 仍处于 `v0.x` 阶段。SDK 优先采用向后兼容的新增 API，并尽量保留旧构造函数和方法；必要的 breaking change 会记录在 [Migration Guide](migration.md) 中。基础路径 `Agent.Run(ctx, input) (string, error)` 会尽量保持稳定，进阶能力通过 `RunDetailed`、`Session`、hooks、policies 和 streaming 逐步扩展。

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

greet := enno.NewTypedToolFromSchema("greet", "Greet a person by name.",
    enno.SchemaObject().
        StringProp("name").
        Required("name"),
    func(ctx context.Context, args GreetArgs) (string, error) {
    return "Hello, " + args.Name + "!", nil
})
```

也可以直接使用 schema builder 生成工具参数定义：

```go
schema := enno.SchemaObject().
    StringProp("query").
    IntegerProp("limit").
    BooleanProp("strict").
    EnumProp("mode", "fast", "safe").
    Required("query", "mode")

search := enno.NewTool("search", "Search records.", schema.Properties(), schema.RequiredFields(),
    func(ctx context.Context, raw json.RawMessage) (string, error) {
        // ...
        return "ok", nil
    },
)
```

`NewAgent` 会在构造时校验工具名、重复工具名和 `required` 字段是否都存在于 `properties` 中，错误会尽早返回。

## 请求选项

`Config.Options` 提供 provider 中立的默认模型调用选项：

```go
temperature := 0.2
strict := true

agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    tools,
    Options: enno.RequestOptions{
        Temperature:     &temperature,
        MaxOutputTokens: 2048,
        ToolChoice:      enno.ToolChoice{Type: enno.ToolChoiceAuto},
        ResponseFormat: enno.ResponseFormat{
            Type:   enno.ResponseFormatJSONSchema,
            Name:   "answer",
            Schema: map[string]any{"type": "object"},
            Strict: &strict,
        },
        Metadata: map[string]string{
            "trace_id": "request-123",
        },
    },
})
```

OpenAI 兼容 provider 会映射 `Temperature`、`MaxOutputTokens`、`ToolChoice`、`ResponseFormat` 和 `Metadata`。Anthropic provider 会映射 `Temperature`、`MaxOutputTokens`、`ToolChoice`、JSON schema `ResponseFormat`，以及 `Metadata["user_id"]`。严格但 provider 不支持的选项会返回 `ErrUnsupportedOption`。

## Hooks

`Config.Hooks` 可以在 provider 调用和工具调用前后控制执行。事件适合观察，hooks 适合修改请求、替换响应、拒绝工具或中止运行。

```go
type approvalHook struct {
    enno.NoopHook
}

func (approvalHook) BeforeToolCall(ctx context.Context, state enno.BeforeToolCallState) (enno.BeforeToolCallResult, error) {
    if state.ToolCall.Name == "bash" {
        return enno.BeforeToolCallResult{
            Deny:        true,
            DenyMessage: "Error: shell tool requires approval",
        }, nil
    }
    return enno.BeforeToolCallResult{}, nil
}

agent, err := enno.NewAgent(enno.Config{
    Provider: provider,
    Tools:    tools,
    Hooks:    []enno.Hook{approvalHook{}},
})
```

`BeforeProviderCall` / `AfterProviderCall` 可以替换 `Request` 或 `Response`，`BeforeToolCall` 可以替换 tool call、拒绝工具或中止运行，`AfterToolCall` 可以替换工具结果或中止运行。

## Streaming

Provider 可以选择实现 `Stream(ctx, Request) (Stream, error)`。SDK 用户调用 `RunStream` 时会收到文本增量、thinking 增量、工具调用增量、usage 和最终响应事件；未实现 streaming 的 provider 会自动回退到 `Complete`，仍返回完整 `RunResult`。

```go
result, err := agent.RunStream(ctx, "总结这个项目", func(ctx context.Context, event enno.StreamEvent) {
    if event.Type == enno.StreamEventTextDelta {
        fmt.Print(event.Text)
    }
})
if err != nil {
    return err
}
fmt.Println(result.StopReason)
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

### NewStructuredTool（结构化结果）

如果工具结果除了模型可见文本外，还需要给宿主应用保留元数据或错误状态，可以使用 `NewStructuredTool`：

```go
lookup := enno.NewStructuredTool("lookup", "Lookup a record.", map[string]any{
    "id": map[string]any{"type": "string"},
}, []string{"id"}, func(ctx context.Context, raw json.RawMessage) (enno.ToolResult, error) {
    var args struct {
        ID string `json:"id"`
    }
    if err := json.Unmarshal(raw, &args); err != nil {
        return enno.ToolResult{}, err
    }
    return enno.ToolResult{
        Content: "record found", // 写入 tool message，模型可见
        Metadata: map[string]any{
            "record_id": args.ID, // 仅保存在 RunResult / Event 中
        },
    }, nil
})
```

`ToolResult.Error` 不会自动改写 `Content`；如果希望模型看到错误，请把错误说明放入 `Content`。旧的 `NewTool` / `NewTypedTool` 返回 `error` 时仍会以 `Error: ...` 文本写回模型，并在 `RunResult` 中标记该工具调用错误。

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
    Workdir:        ".",
    Timeout:        120 * time.Second,
    MaxOutputChars: 50000,
    SafetyPolicy:   shell.SafetyPolicyDenyList, // 默认值
}))
```

提供 `bash` 工具，受 `Workdir`、`Timeout`、`MaxOutputChars` 和 `SafetyPolicy` 约束。`SafetyPolicyDenyList` 是默认值；需要替换默认 denylist 时仍可设置 `DenyList`。

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

需要查看 usage、停止原因、每轮工具调用和最终消息时，使用 `RunDetailed`：

```go
result, err := agent.RunDetailed(ctx, "总结当前项目")
if err != nil {
    return err
}
fmt.Println(result.Content, result.StopReason, result.Usage)
```

需要显式管理对话状态时，使用 `Session` 和 `RunSession`。这适合 HTTP 服务、Bot、桌面应用等需要把会话加载、保存或分叉的场景。

### 无隐藏历史的请求处理

```go
sessions := map[string]*enno.Session{}

func handle(ctx context.Context, userID string, input string) (string, error) {
    session := sessions[userID]
    if session == nil {
        session = &enno.Session{}
        sessions[userID] = session
    }

    result, err := agent.RunSession(ctx, session, input)
    if err != nil {
        return "", err
    }
    return result.Content, nil
}
```

### 加载和保存 Session JSON

`Session.Messages` 是可序列化字段，可以按应用自己的存储方式落盘或写入数据库：

```go
var session enno.Session

if data, err := os.ReadFile("session.json"); err == nil {
    if err := json.Unmarshal(data, &session); err != nil {
        return err
    }
}

result, err := agent.RunSession(ctx, &session, "继续总结这个项目")
if err != nil {
    return err
}

data, err := json.MarshalIndent(session, "", "  ")
if err != nil {
    return err
}
if err := os.WriteFile("session.json", data, 0o600); err != nil {
    return err
}
fmt.Println(result.Content)
```

### 分叉 Session 做试探性运行

```go
branch := session.Clone()

result, err := agent.RunSession(ctx, &branch, "换一种方案评估风险")
if err != nil {
    return err
}

// 原 session 不会被修改；确认采用时再替换。
session = branch
fmt.Println(result.Content)
```

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
| `github.com/dean2021/enno/tools/filesystem` | 受根目录限制的文件读写编辑工具 |
| `github.com/dean2021/enno/tools/shell` | 受工作目录、超时、输出上限和 safety policy 限制的 shell 工具 |
| `github.com/dean2021/enno/tools/grep` | ripgrep 内容搜索工具 |
| `github.com/dean2021/enno/tools/glob` | ripgrep 文件名 glob 工具 |
| `github.com/dean2021/enno/tools/subagent` | 子 Agent 委派工具 |
| `github.com/dean2021/enno/tools/loadskill` | SKILL.md 加载与检索工具 |
| `github.com/dean2021/enno/tools/compact` | 上下文压缩触发工具 |

## Built-In Tool Options

内置工具使用一致的默认约定：超时默认 120 秒，长输出默认最多 50000 字符并追加 `[truncated]`。`filesystem`、`shell`、`grep`、`glob` 和 `subagent` 都可以通过各自 Config 设置输出上限。

```go
tools := filesystem.New(filesystem.Config{
    Root:           ".",
    MaxOutputChars: 20000,
})

bash := shell.New(shell.Config{
    Workdir:        ".",
    Timeout:        30 * time.Second,
    MaxOutputChars: 20000,
    SafetyPolicy:   shell.SafetyPolicyDenyList,
})
```

`shell.SafetyPolicyDenyList` 是默认值，会使用 denylist 拦截明显危险命令；确需完全自定义安全策略时可以使用 hooks 或显式设置 `SafetyPolicyAllowAll` 后在宿主侧自行审批。

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
