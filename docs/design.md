# Enno Agent Framework Design

## 目标

Enno 的目标是提供一个可被 Go 项目直接引入的通用 Agent 框架，同时交付一个可安装使用的 CLI Agent。框架核心只负责 Agent 循环、消息历史、工具调用和模型供应商抽象；具体模型 SDK、内置工具和 CLI 配置都放在独立包中。

设计重点：

- 根包 `enno` 提供稳定公共 API，不暴露 OpenAI 或 Anthropic SDK 类型。
- provider 以插件形式接入，新增模型供应商不需要改 Agent loop。
- tools 以可选包形式组合，用户可以只使用框架核心，也可以引入内置文件、shell、todo 工具。
- CLI 复用库能力，只负责读取参数、组装 provider/tools、启动内部 UI 或执行一次 `Agent.Run`。

## 目录结构

```text
enno/
  agent.go
  config.go
  errors.go
  message.go
  provider_iface.go
  tool.go

  provider/
    provider.go
    openai/
      openai.go
    anthropic/
      anthropic.go

  tools/
    todo/
      todo.go
    filesystem/
      filesystem.go
    shell/
      shell.go

  internal/
    cliconfig/
      config.go
    cliui/
      repl.go

  cmd/
    enno/
      main.go

  examples/
```

## 核心包职责

### 根包 `enno`

根包是用户最常接触的 API 面：

- `Agent`：维护对话历史、执行 Agent loop、分发工具调用。
- `Config`：注入 provider、system prompt、tools、最大工具轮数。
- `Provider`：模型供应商统一接口。
- `Request` / `Response`：Agent 与 provider 之间的统一协议。
- `Message` / `ToolCall`：跨 provider 的统一消息和工具调用结构。
- `Tool`：工具声明和本地执行 handler 的统一表示。

根包不读取环境变量，也不导入具体 provider SDK。这样它可以在服务端、测试、嵌入式场景中稳定复用。

Agent 支持可选事件回调，用于观察模型调用、工具调用、工具结果和 token usage。事件只包含可观测执行过程和模型显式返回内容，不包含隐藏 chain-of-thought。

### `provider/*`

provider 子包负责把 `enno.Request` 翻译成具体模型 SDK 请求，并把响应翻译回 `enno.Response`。

- `provider/openai`：OpenAI Chat Completions 兼容实现，支持 OpenAI 兼容网关。
- `provider/anthropic`：Anthropic Messages API 实现，支持 `tool_use` 和 `tool_result`。
- `provider/provider.go`：提供 `enno.Provider` 等类型别名，方便用户按 provider 目录理解接口。

新增 provider 时，应实现：

```go
type Provider struct {
    // SDK client and config
}

func (p *Provider) Complete(ctx context.Context, req enno.Request) (enno.Response, error)
```

### `tools/*`

内置工具是普通的 `enno.Tool`，不享有特殊权限：

- `tools/todo`：提供 `todo` 工具，每次 `todo.New()` 都拥有独立状态。
- `tools/filesystem`：提供 `read_file`、`write_file`、`edit_file`，通过 `Config.Root` 限制文件访问范围。
- `tools/shell`：提供 `bash`，通过 `Config.Workdir`、`Config.Timeout`、`Config.DenyList` 控制执行环境。

内置工具不默认注入根包 `Agent`，调用方需要显式选择。

### `internal/cliui`

`internal/cliui` 是 CLI 专用的终端 UI 层，负责 `cmd/enno` 基于 `tview` 的交互式 TUI 和非终端 fallback。它消费 Agent 事件来展示运行状态、工具轨迹和上下文使用情况。

它不是公共 SDK API。SDK 用户应直接调用 `Agent.Run(ctx, input)`，并在自己的 HTTP、Bot、桌面端或终端应用中自行组织交互层。

### `cmd/enno`

CLI 是正式交付物，但保持薄封装：

- 读取 CLI flags 和 `config.yaml`。
- 构造 provider。
- 注册默认工具。
- `run` 模式直接调用 `Agent.Run`。
- 交互模式调用 `internal/cliui`。

CLI 专用配置逻辑放在 `internal/cliconfig`，避免污染库包。

## 数据流

```mermaid
flowchart TD
    userCode[User Code] --> agent[enno.Agent]
    cli[cmd/enno CLI] --> cliConfig[internal/cliconfig]
    cli --> cliUI[internal/cliui]
    cliUI --> agent
    cli --> agent
    agent --> providerIface[enno.Provider]
    providerIface --> openaiProvider[provider/openai]
    providerIface --> anthropicProvider[provider/anthropic]
    agent --> toolRegistry[Tool Registry]
    toolRegistry --> builtinTools[tools Packages]
    toolRegistry --> customTools[User Tools]
```

## Agent Loop

`Agent.Run(ctx, input)` 的流程：

1. 将用户输入追加到 `Agent` 的历史消息。
2. 调用 `Provider.Complete`，传入 system prompt、历史和工具定义。
3. 如果 provider 返回普通文本，追加 assistant message 并返回文本。
4. 如果 provider 返回 tool calls，逐个查找本地工具并执行。
5. 将工具结果追加为 tool message，继续下一轮模型调用。
6. 达到 `MaxToolRounds` 后返回错误，防止无限工具循环。

仅当 `Config.Tools` 中注册了名为 `todo` 的工具时，`Agent` 才会在多轮工具执行后跟踪「距离上次调用 todo 的轮数」：连续 **3** 轮模型回合里都执行了工具但未调用 `todo` 时，会在历史中追加一条内容为 `<reminder>Update your todos.</reminder>` 的用户消息，促使模型更新任务列表。未挂载 `todo` 工具时不会注入该提醒。

`Agent` 内部持有互斥锁，同一个实例的 `Run` 调用会串行执行。需要并发会话时，应创建多个 `Agent` 实例。

## 扩展点

### 自定义工具

使用 `enno.NewTool` 可以直接接收原始 JSON 参数；使用 `enno.NewTypedTool[T]` 可以让框架完成 JSON 解析：

```go
type Input struct {
    Name string `json:"name"`
}

tool := enno.NewTypedTool("greet", "Greet a person.", schema, []string{"name"},
    func(ctx context.Context, input Input) (string, error) {
        return "Hello, " + input.Name, nil
    },
)
```

### 自定义 Provider

只要实现 `enno.Provider` 即可接入任意模型后端：

```go
type MyProvider struct{}

func (p *MyProvider) Complete(ctx context.Context, req enno.Request) (enno.Response, error) {
    // Convert req to your model API, then return enno.Response.
}
```

## 设计约束

- 根包不得导入具体模型 SDK。
- CLI 不得实现独立 Agent loop。
- 内置工具不得依赖全局状态。
- 文件和 shell 工具必须显式配置工作目录或根目录。
- provider adapter 只做协议转换，不执行本地工具。
