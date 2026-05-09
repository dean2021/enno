# Enno SDK 使用指南

本文档介绍如何将 Enno 作为 Go Package 嵌入到自己的项目中。CLI 使用请参考 [CLI 使用指南](usage-cli.md)。

## SDK 稳定性

Enno 仍处于 `v0.x` 阶段。SDK API 以显式 `Session` 和结构化 `RunResult` 为核心；breaking change 会记录在 [Migration Guide](migration.md) 中。基础路径 `Agent.Run(ctx, session, input) (RunResult, error)` 会尽量保持稳定，进阶能力通过 hooks、policies 和 streaming 逐步扩展。

SDK 不内置 agent identity。`SystemPrompt` 和 `SystemPromptSections` 由调用方完全控制，适合注入 Identity、Rules、Domain Context、Output Style 等应用层内容。CLI 那套 coding-agent section、环境、git、项目规则和工具指导是 `internal/systemprompt.NewCodingAgent` / `internal/projectrules` 的装配逻辑，不会自动进入纯 SDK 用法。SDK 只会通过 runtime section 追加通用能力说明，例如 `sdk.BuiltinTools.LoadSkill` 的 skills 摘要。

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
    "github.com/dean2021/enno/sdk"
)

func main() {
    provider, err := openaiprovider.New(openaiprovider.Config{
        APIKey:  os.Getenv("ENNO_API_KEY"),
        BaseURL: os.Getenv("ENNO_BASE_URL"),
        Model:   os.Getenv("ENNO_MODEL"),
    })
    if err != nil {
        panic(err)
    }

    agent, err := sdk.NewAgent(sdk.Config{
        Provider:     provider,
        SystemPrompt: "Follow the application-provided sections below.",
        SystemPromptSections: []sdk.SystemPromptSection{
            {Name: "Identity", Content: "You are a helpful coding agent."},
            {Name: "Output Style", Content: "Be concise and concrete."},
        },
		BuiltinTools: sdk.BuiltinTools{
			TaskGraph:  &sdk.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
			Filesystem: &sdk.FilesystemTool{Root: "."},
			Grep:       &sdk.GrepTool{Root: "."},
			Glob:       &sdk.GlobTool{Root: "."},
			FetchURL:   &sdk.FetchURLTool{Timeout: 30 * time.Second},
		},
	})
	if err != nil {
		panic(err)
	}

    session := &enno.Session{}
    result, err := agent.Run(context.Background(), session, "查看当前目录有哪些文件")
    if err != nil {
        panic(err)
    }
    fmt.Println(result.Content)
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

如果你希望复用 CLI 风格的 system prompt 组织方式，可以在自己的应用中使用 `sdk.Config.SystemPromptSections` 组合多段内容。Enno 不会替你隐式读取 `CLAUDE.md`、git 状态或工作目录外的项目规则。

`SystemPrompt` 会先输出，随后按顺序输出 `SystemPromptSections`，最后追加 SDK 自动生成的能力说明 section（目前主要是 skills 摘要）：

```go
agent, err := sdk.NewAgent(sdk.Config{
    Provider:     provider,
    SystemPrompt: "Follow the application-provided sections below.",
    SystemPromptSections: []sdk.SystemPromptSection{
        {Name: "Identity", Content: "You are a repository maintenance agent."},
        {Name: "Rules", Content: "Read relevant files before proposing changes."},
        {Name: "Output Style", Content: "Prefer short, actionable answers."},
    },
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
agent, err := sdk.NewAgent(sdk.Config{
    Provider:     provider,
    SystemPrompt: "Follow the application-provided sections below.",
    SystemPromptSections: []sdk.SystemPromptSection{
        {Name: "Identity", Content: "You are a helpful agent."},
    },
    BuiltinTools: sdk.BuiltinTools{
        TaskGraph: &sdk.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
    },
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

agent, err := sdk.NewAgent(sdk.Config{
    Provider: provider,
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

agent, err := sdk.NewAgent(sdk.Config{
    Provider: provider,
    Hooks:    []enno.Hook{approvalHook{}},
})
```

`BeforeProviderCall` / `AfterProviderCall` 可以替换 `Request` 或 `Response`，`BeforeToolCall` 可以替换 tool call、拒绝工具或中止运行，`AfterToolCall` 可以替换工具结果或中止运行。

## Streaming

Provider 可以选择实现 `Stream(ctx, Request) (Stream, error)`。SDK 用户调用 `RunStream` 时会收到文本增量、thinking 增量、工具调用增量、usage 和最终响应事件；未实现 streaming 的 provider 会自动回退到 `Complete`，仍返回完整 `RunResult`。

```go
result, err := agent.RunStream(ctx, session, "总结这个项目", func(ctx context.Context, event enno.StreamEvent) {
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
agent, err := sdk.NewAgent(sdk.Config{
    Provider:    provider,
    CustomTools: []enno.Tool{greet},
})
```

## 内置工具

内置工具不再作为公开 `tools/*` 包暴露。SDK 用户通过高层 `sdk.Config.BuiltinTools` 启用、禁用和配置内置工具；`enno.Tool` 只用于自定义工具。

```go
agent, err := sdk.NewAgent(sdk.Config{
    Provider:     provider,
    SystemPrompt: "Follow the application-provided sections below.",
    SystemPromptSections: []sdk.SystemPromptSection{
        {Name: "Identity", Content: "You are a helpful coding agent."},
    },
    BuiltinTools: sdk.BuiltinTools{
        TaskGraph:  &sdk.TaskGraphTool{Root: ".", Timeout: 120 * time.Second},
        Filesystem: &sdk.FilesystemTool{Root: ".", Read: true, Write: false},
        Shell:      nil, // disabled
        Grep:       &sdk.GrepTool{Root: ".", Timeout: 120 * time.Second},
        Glob:       &sdk.GlobTool{Root: ".", Timeout: 120 * time.Second},
        FetchURL:   &sdk.FetchURLTool{Timeout: 30 * time.Second},
        LoadSkill:  &sdk.LoadSkillTool{Dirs: []string{os.Getenv("HOME") + "/.enno/skills"}},
        Subagent:   &sdk.SubagentTool{},
    },
    Permissions: sdk.ToolPermissions{
        Mode:            sdk.PermissionAllow,
        AllowedTools:    []string{"read_file", "grep", "glob", "fetch_url", "task_create", "task_update", "task_list", "task_get"},
        DisallowedTools: []string{"bash", "write_file", "edit_file"},
    },
    Compaction: &enno.CompactionConfig{
        Enabled: true,
    },
})
```

可用内置工具：

- `TaskGraph`：注册 `task_create` / `task_update` / `task_list` / `task_get`。
- `Filesystem`：注册 `read_file`，并可按 `Write` 控制 `write_file` / `edit_file`。
- `Shell`：注册 `bash`，受 `Workdir`、`Timeout`、`MaxOutputChars` 和 `SafetyPolicy` 控制。
- `Grep` / `Glob`：通过系统 `rg` 搜索内容或列出文件，需本机安装 ripgrep。
- `FetchURL`：注册 `fetch_url`，读取 HTTP/HTTPS URL 并将 HTML 转成可读 markdown。
- `LoadSkill`：扫描 `SKILL.md` 目录并注册 `load_skill`。
- `Subagent`：注册隔离子 Agent；子 Agent 自动获得同一组子工具和工具权限，且不递归包含 `subagent`。
- `Compact` / `Compaction`：启用手动 `compact` 和自动上下文压缩。

`Permissions` 是执行权限层，不是工具注册层。未启用的内置工具不会暴露给模型；已注册工具仍可通过 `AllowedTools` / `DisallowedTools` 限制执行，且 `DisallowedTools` 优先。

## 执行 Agent

SDK 用户直接调用 `Agent.Run`，并传入显式 `Session`：

```go
session := &enno.Session{}
result, err := agent.Run(ctx, session, "总结当前项目")
if err != nil {
    return err
}
fmt.Println(result.Content, result.StopReason, result.Usage)
```

单次 `Run` 内 `MaxToolRounds` 默认为 0（不限制工具/模型轮数）；需要硬上限时在 `enno.Config` 中设置正整数。

`Agent` 内部持有互斥锁，同一实例的 `Run` 调用串行执行。并发会话请创建多个 `Agent` 实例。

`Session` 适合 HTTP 服务、Bot、桌面应用等需要把会话加载、保存或分叉的场景。

### 无隐藏历史的请求处理

```go
sessions := map[string]*enno.Session{}

func handle(ctx context.Context, userID string, input string) (string, error) {
    session := sessions[userID]
    if session == nil {
        session = &enno.Session{}
        sessions[userID] = session
    }

    result, err := agent.Run(ctx, session, input)
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

result, err := agent.Run(ctx, &session, "继续总结这个项目")
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

result, err := agent.Run(ctx, &branch, "换一种方案评估风险")
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
agent, err := sdk.NewAgent(sdk.Config{
    Provider: provider,
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
| `github.com/dean2021/enno/sdk` | 高层 SDK 装配、内置工具配置与权限控制 |
| `github.com/dean2021/enno/provider/openai` | OpenAI Chat Completions 兼容 provider |
| `github.com/dean2021/enno/provider/anthropic` | Anthropic Messages API provider |

## Built-In Tool Options

内置工具使用一致的默认约定：本地工具超时默认 120 秒，`FetchURL` 默认 30 秒；长输出默认最多 50000 字符并追加 `[truncated]`。通过 `sdk.BuiltinTools` 中各工具配置设置输出上限和安全策略。

```go
agent, err := sdk.NewAgent(sdk.Config{
    Provider: provider,
    BuiltinTools: sdk.BuiltinTools{
        Filesystem: &sdk.FilesystemTool{
            Root: ".",
            Read: true,
            Write: false,
            MaxOutputChars: 20000,
        },
        Shell: &sdk.ShellTool{
            Workdir: ".",
            Timeout: 30 * time.Second,
            MaxOutputChars: 20000,
            SafetyPolicy: sdk.ShellSafetyPolicyDenyList,
        },
        FetchURL: &sdk.FetchURLTool{
            Timeout: 30 * time.Second,
            MaxOutputChars: 20000,
        },
    },
})
```

`sdk.ShellSafetyPolicyDenyList` 是默认值，会使用 denylist 拦截明显危险命令；确需完全自定义安全策略时可以使用 hooks 或显式设置 `sdk.ShellSafetyPolicyAllowAll` 后在宿主侧自行审批。

## 安全建议

- 生产环境不要默认开启 `sdk.ShellTool`，除非运行环境有隔离措施。
- `sdk.FilesystemTool` 必须指定合适的 `Root`，避免模型访问不该访问的路径。
- API key 应通过环境变量或密钥管理系统注入，不要写入代码。
- 每个独立对话使用独立 `Session`；需要并行运行或隔离工具状态时再创建独立 `Agent` 实例。

## 示例

仓库内置示例：

```sh
go run ./examples/sdk_walkthrough
go run ./examples/simple_agent
go run ./examples/custom_tool
go run ./examples/anthropic
```

这些示例分别展示完整离线 SDK 流程、基础 Agent、自定义工具和 Anthropic provider 的用法。
