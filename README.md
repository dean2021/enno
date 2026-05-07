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
- Optional Agent events for observing model calls, tool calls, results, and token usage.
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
go install github.com/dean2021/enno/cmd/enno@v0.3.0
go get github.com/dean2021/enno@v0.3.0
```

## CLI Usage

Start the `tview` terminal UI interactive mode:

```sh
enno
```

Type a task and press Enter. Use `Esc`, `Ctrl+C`, `q`, or `exit` to leave the interactive UI. The UI shows observable progress such as model calls, tool calls, tool results, message counts, and token usage when providers return it. It does not display hidden model chain-of-thought.

Run a single prompt:

```sh
enno run "Analyze this repository"
```

Configure the CLI in `~/.enno/config.yaml`. If the default config file does not exist, Enno creates a commented template on startup:

```yaml
provider: openai
model: your-model
api_key: your-key
base_url: https://example.com/v1
max_tokens: 4096
shell: true
filesystem: true
```

Use a custom config file:

```sh
enno --config /path/to/config.yaml
enno run --config /path/to/config.yaml "Analyze this repository"
```

Common flags:

```sh
enno --workdir .
enno --no-shell
enno --no-filesystem
```

Provider, model, API key, base URL, and max token settings are read from `config.yaml` only. The CLI does not read `ENNO_*` environment variables for these values.

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

## Observability

Attach an optional event handler to observe model calls, tool calls, tool results, and token usage:

```go
agent, err := enno.NewAgent(enno.Config{
	Provider: provider,
	Tools:    tools,
	EventHandler: func(ctx context.Context, event enno.Event) {
		fmt.Printf("%s round=%d usage=%+v\n", event.Type, event.Round, event.Usage)
	},
})
```

Events expose observable execution details and model-visible content only. They do not expose hidden model chain-of-thought.

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
  internal/cliui        CLI-only terminal UI
  internal/cliconfig    CLI-only configuration parsing
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

`make tag` creates a Git tag for the version in `VERSION`, such as `v0.3.0`. Pushing the tag triggers the release workflow.

## Safety Notes

- Avoid enabling `tools/shell` in production without sandboxing.
- Always configure `tools/filesystem` with a restricted root directory.
- Do not hard-code API keys in source code.
- Use separate `Agent` instances for independent user sessions.

## License

Enno is released under the [MIT License](LICENSE).
