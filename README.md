# Enno

Enno is a lightweight Go agent framework that can be embedded as a package or installed as a CLI agent.

It provides a provider-agnostic Agent loop, a composable tool system, built-in OpenAI-compatible and Anthropic providers, and optional tools for todo tracking, filesystem access, and shell execution.

Repository: [github.com/dean2021/enno](https://github.com/dean2021/enno)

## Features

- Provider-neutral core package: `Agent`, `Provider`, `Tool`, `Message`, `Request`, and `Response`.
- OpenAI-compatible provider via `provider/openai`.
- Anthropic Messages API provider via `provider/anthropic`.
- Optional built-in tools:
  - `tools/todo`
  - `tools/filesystem`
  - `tools/shell`
- Reusable runners:
  - `runner.REPL`
  - `runner.Once`
- Installable CLI at `cmd/enno`.
- Extensible tool and provider interfaces for custom integrations.

## Installation

Install the CLI:

```sh
go install github.com/dean2021/enno/cmd/enno@latest
```

Use as a Go package:

```sh
go get github.com/dean2021/enno
```

Install a specific version:

```sh
go install github.com/dean2021/enno/cmd/enno@v0.1.0
go get github.com/dean2021/enno@v0.1.0
```

## CLI Usage

Start interactive mode:

```sh
enno
```

Run a single prompt:

```sh
enno run "Analyze this repository"
```

Use Anthropic:

```sh
export ENNO_PROVIDER=anthropic
export ANTHROPIC_API_KEY=your-key
export ENNO_MODEL=claude-sonnet-4-5-20250929
enno
```

Use an OpenAI-compatible endpoint:

```sh
export ENNO_PROVIDER=openai
export ENNO_API_KEY=your-key
export ENNO_BASE_URL=https://example.com/v1
export ENNO_MODEL=your-model
enno
```

Common flags:

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

Configuration priority:

1. CLI flags
2. Environment variables
3. Defaults

## Package Usage

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
			BaseURL: os.Getenv("ENNO_BASE_URL"),
			Model:   os.Getenv("ENNO_MODEL"),
		}),
		SystemPrompt: "You are a helpful coding agent.",
		Tools:        tools,
	})
	if err != nil {
		panic(err)
	}

	answer, err := agent.Run(context.Background(), "List the files in this workspace.")
	if err != nil {
		panic(err)
	}
	fmt.Println(answer)
}
```

## Anthropic Provider

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

## Custom Tools

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

Then pass the tool to an agent:

```go
agent, err := enno.NewAgent(enno.Config{
	Provider: provider,
	Tools:    []enno.Tool{greet},
})
```

## Architecture

```text
enno/
  agent.go              core Agent loop
  config.go             Agent configuration
  message.go            provider-neutral messages
  tool.go               tool declaration and execution API
  provider_iface.go     provider interface

  provider/openai       OpenAI-compatible provider
  provider/anthropic    Anthropic provider
  tools/todo            todo tool
  tools/filesystem      file read/write/edit tools
  tools/shell           shell tool
  runner                REPL and one-shot runners
  cmd/enno              installable CLI
  examples              usage examples
  docs                  design and usage documentation
```

See:

- [Design document](docs/design.md)
- [Usage guide](docs/usage.md)
- [Release guide](docs/release.md)
- [Changelog](CHANGELOG.md)

## Versioning

Enno follows Semantic Versioning. The current initial release line is `v0.x.y` while the public API is still evolving.

Useful release commands:

```sh
make help
make version
make release-check
make tag
```

`make tag` creates a Git tag for the version in `VERSION`, such as `v0.1.0`. Pushing the tag triggers the release workflow.

## Safety Notes

- Avoid enabling `tools/shell` in production without sandboxing.
- Always configure `tools/filesystem` with a restricted root directory.
- Do not hard-code API keys in source code.
- Use separate `Agent` instances for independent user sessions.

## License

Enno is released under the [MIT License](LICENSE).
