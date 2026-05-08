# Enno

Enno is a lightweight Go agent framework that can be embedded as a package or installed as a CLI agent.

It provides a provider-agnostic Agent loop, a composable tool system, built-in OpenAI-compatible and Anthropic providers, and optional tools for a persistent **task graph** (`task_create` / `task_update` / `task_list` / `task_get`), filesystem access, shell execution, ripgrep-based content search (`grep`), and ripgrep-based file globbing (`glob`).

Repository: [github.com/dean2021/enno](https://github.com/dean2021/enno)

## Features

- Provider-neutral core package: `Agent`, `Provider`, `Tool`, `Message`, `Request`, and `Response`.
- OpenAI-compatible provider via `provider/openai` (HTTP retries with backoff for 429/5xx; default retry budget raised above the SDK default for flaky gateways; optional fixed HTTP proxy via config or `HTTPProxy` field).
- Anthropic Messages API provider via `provider/anthropic` (same retry behavior and optional proxy).
- Optional built-in tools:
  - `tools/taskgraph` (DAG task tools; CLI stores under `~/.enno/tasks/<session_id>/`, default on, disable with `task_graph: false` or `--no-task-graph`)
  - `tools/filesystem`
  - `tools/shell`
  - `tools/grep` (`grep`: regex search via system `rg`; CLI default on, disable with `grep: false` or `--no-grep`)
  - `tools/glob` (`glob`: file patterns via `rg --files`; CLI default on, disable with `glob: false` or `--no-glob`)
  - `tools/subagent` (`subagent` tool: isolated child agent; CLI enables via `subagent: true` in config)
  - `tools/loadskill` (`load_skill` + `SKILL.md` trees; CLI: `skills_dir` in config or `--skills-dir`)
  - `tools/compact` + `Config.Compaction`: optional context compression (micro tool-result trimming, auto summarization, manual `compact`); default off, configured via YAML or struct
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
go install github.com/dean2021/enno/cmd/enno@latest
go get github.com/dean2021/enno@latest
```

## CLI Usage

Start the bubbletea (Charm) terminal UI interactive mode:

```sh
enno
```

Type a task and press Enter. Use `Esc`, `Ctrl+C`, `q`, or `exit` to leave the interactive UI. The main `Enno` window shows a single conversation stream: user prompts, model progress, tool calls, tool arguments, muted tool results, and final answers are appended in order so current activity stays visible. **Tab** switches focus between the prompt and the transcript; with focus on the transcript, use arrow keys, **PgUp/PgDn**, **Home/End**, **gg**/**G**, and **/** or **Ctrl+F** for jump search; with focus on the prompt, **Alt+↑/↓** browses input history and **Ctrl+↑/↓** scrolls the transcript. The mouse wheel scrolls the transcript even when the prompt is focused (no need to Tab first); wheel also works while the jump-search overlay is open. Because mouse tracking captures all mouse events, text selection requires **Shift+drag** in all terminals. It does not display hidden model chain-of-thought.

Run a single prompt:

```sh
enno run "Analyze this repository"
```

Configure the CLI in `~/.enno/config.yaml`. A commented reference copy lives at [`config.yaml.example`](config.yaml.example) in this repository. Skills load from `~/.enno/skills` by default (merge with optional `skills_extra_dirs` and `--skills-dir`). Set `subagent: true` to register the `subagent` tool (isolated child agent per delegation). If the default config file does not exist, Enno creates a commented template on startup:

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

	answer, err := agent.Run(context.Background(), "List the files in this workspace.")
	if err != nil {
		panic(err)
	}
	fmt.Println(answer)
}
```

## Anthropic Provider

```go
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

The CLI renders these events directly in the main `Enno` conversation stream, similar to a coding-agent transcript: model progress, tool calls, parameters, and muted results appear inline as they happen.

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
  tools/taskgraph       persistent DAG task tools (task_*)
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
- [SDK usage guide](docs/usage-sdk.md)
- [CLI usage guide](docs/usage-cli.md)
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

`make tag` creates a Git tag for the version in `VERSION`, such as `v0.4.0`. Pushing the tag triggers the release workflow.

## Safety Notes

- Avoid enabling `tools/shell` in production without sandboxing.
- Always configure `tools/filesystem` with a restricted root directory.
- Do not hard-code API keys in source code.
- Use separate `Agent` instances for independent user sessions.

## License

Enno is released under the [MIT License](LICENSE).
