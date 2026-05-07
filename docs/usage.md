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

进入 REPL 后输入任务，输入 `q` 或 `exit` 退出。

### 单次执行

```sh
enno run "帮我分析当前目录的 Go 包结构"
```

### 切换 Provider

默认 provider 是 OpenAI 兼容接口。可以通过 flags 指定 Anthropic：

```sh
enno --provider anthropic --model claude-sonnet-4-5-20250929
```

也可以通过环境变量：

```sh
export ENNO_PROVIDER=anthropic
export ANTHROPIC_API_KEY=your-key
export ENNO_MODEL=claude-sonnet-4-5-20250929
enno
```

### OpenAI 兼容网关

```sh
export ENNO_PROVIDER=openai
export ENNO_API_KEY=your-key
export ENNO_BASE_URL=https://example.com/v1
export ENNO_MODEL=your-model
enno
```

### 常用 Flags

```sh
enno --provider openai
enno --provider anthropic
enno --model astron-code-latest
enno --base-url https://example.com/v1
enno --workdir .
enno --no-shell
enno --no-filesystem
enno --max-tokens 4096
```

配置优先级：

1. 命令行 flags
2. 环境变量
3. 默认值

## 作为 Go Package 使用

### 基础用法

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/dean2021/enno"
    openaiprovider "github.com/dean2021/enno/provider/openai"
    "github.com/dean2021/enno/tools/filesystem"
    "github.com/dean2021/enno/tools/todo"
)

func main() {
    tools := []enno.Tool{todo.New()}
    tools = append(tools, filesystem.New(filesystem.Config{Root: "."})...)

    agent, err := enno.NewAgent(enno.Config{
        Provider: openaiprovider.New(openaiprovider.Config{
            APIKey:  os.Getenv("ENNO_API_KEY"),
            BaseURL: "https://maas-coding-api.cn-huabei-1.xf-yun.com/v2",
            Model:   "astron-code-latest",
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
    Tools:        []enno.Tool{todo.New()},
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

### 使用内置工具

Todo 工具：

```go
tools := []enno.Tool{todo.New()}
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

### 使用 Runner

交互式 REPL：

```go
err := runner.REPL(ctx, agent, runner.Config{
    Prompt: "enno >> ",
    In:     os.Stdin,
    Out:    os.Stdout,
    Err:    os.Stderr,
})
```

单次执行：

```go
answer, err := runner.Once(ctx, agent, "总结当前项目")
```

## 包说明

- `github.com/dean2021/enno`：核心 Agent API。
- `github.com/dean2021/enno/provider/openai`：OpenAI Chat Completions 兼容 provider。
- `github.com/dean2021/enno/provider/anthropic`：Anthropic Messages API provider。
- `github.com/dean2021/enno/tools/todo`：任务列表工具。
- `github.com/dean2021/enno/tools/filesystem`：受根目录限制的文件读写编辑工具。
- `github.com/dean2021/enno/tools/shell`：受工作目录和 denylist 限制的 shell 工具。
- `github.com/dean2021/enno/runner`：REPL 和单次执行封装。
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
