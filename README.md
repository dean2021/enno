# Enno

Enno is a lightweight Go agent framework that can be embedded as a package or installed as a CLI agent.

It provides a provider-agnostic Agent loop, a composable tool system, built-in OpenAI-compatible and Anthropic providers, and optional tools for a persistent **task graph** (`task_create` / `task_update` / `task_list` / `task_get`), filesystem access, shell execution, ripgrep-based search (`grep` / `glob`), and URL fetching (`fetch_url`).

The CLI assembles its system prompt from named sections for identity, safety, environment, project instructions, communication, and skills. Tool-specific usage guidance lives in each tool description. SDK users define their own identity through `SystemPrompt` or `SystemPromptSections`.

Repository: [github.com/dean2021/enno](https://github.com/dean2021/enno)

## Features

- Provider-neutral core package: `Agent`, `Session`, `RunResult`, `Provider`, `Tool`, `Message`, `Request`, and `Response`.
- OpenAI-compatible provider via `provider/openai` (HTTP retries with backoff for 429/5xx; default retry budget raised above the SDK default for flaky gateways; optional fixed HTTP proxy via config or `HTTPProxy` field).
- Anthropic Messages API provider via `provider/anthropic` (same retry behavior and optional proxy).
- High-level `sdk` package for configuring built-in tools without importing their implementations: task graph, filesystem, shell, grep, glob, fetch_url, subagent, load_skill, compact, and allow/deny tool permissions.
- Optional Agent events for observing model calls, tool calls, results, and token usage.
- Installable CLI at `cmd/enno`.
- Extensible tool and provider interfaces for custom integrations.

## Installation

Install the CLI:

```sh
go install github.com/dean2021/enno/cmd/enno@latest
```

The CLI currently ships from this repository. It is planned to move to a
standalone CLI module later; the SDK package path will remain
`github.com/dean2021/enno`, and the future CLI module will depend on it through
public packages only.

Use as a Go package:

```sh
go get github.com/dean2021/enno
```

Install a specific version:

```sh
go install github.com/dean2021/enno/cmd/enno@v0.9.0
go get github.com/dean2021/enno@v0.9.0
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
	result, err := agent.Run(context.Background(), session, "List the files in this workspace.")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Content)
}
```

`Run` returns detailed usage, stop reason, rounds, tool calls, and the messages
produced by the run:

```go
result, err := agent.Run(context.Background(), session, "List the files in this workspace.")
if err != nil {
	panic(err)
}
fmt.Printf("answer=%s rounds=%d usage=%+v stop=%s\n",
	result.Content, len(result.Rounds), result.Usage, result.StopReason)
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

## Custom Tools

```go
type GreetArgs struct {
	Name string `json:"name"`
}

greet := enno.NewTypedToolFromSchema("greet", "Greet a person by name.", enno.SchemaObject().
	StringProp("name").
	Required("name"), func(ctx context.Context, args GreetArgs) (string, error) {
	return "Hello, " + args.Name + "!", nil
})
```

Then pass the tool to an agent:

```go
agent, err := sdk.NewAgent(sdk.Config{
	Provider:    provider,
	CustomTools: []enno.Tool{greet},
})
```

## Observability

Attach an optional event handler to observe model calls, tool calls, tool results, and token usage:

```go
agent, err := sdk.NewAgent(sdk.Config{
	Provider: provider,
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
  run_result.go         detailed run result types
  session.go            explicit conversation state
  stream.go             streaming interfaces and events
  request_options.go    provider-neutral request options
  hooks.go              lifecycle control hooks
  policy.go             loop policies
  config.go             Agent configuration
  message.go            provider-neutral messages
  tool.go               tool declaration and execution API
  provider_iface.go     provider interface

  provider/openai       OpenAI-compatible provider
  provider/anthropic    Anthropic provider
  provider/internal     Provider-shared implementation helpers
  sdk                   high-level SDK assembler and built-in tool config
  internal/builtintools internal built-in tool implementations
  internal/cliui        CLI-only terminal UI, moves with the CLI project later
  internal/cliconfig    CLI-only configuration parsing, moves with the CLI project later
  internal/history      CLI history recorder and reader, moves with the CLI project later
  cmd/enno              installable CLI
  examples              usage examples
  docs                  design and usage documentation
```

See:

- [Design document](docs/design.md)
- [SDK usage guide](docs/usage-sdk.md)
- [CLI usage guide](docs/usage-cli.md)
- [Release guide](docs/release.md)
- [Migration guide](docs/migration.md)
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

`make tag` creates a Git tag for the version in `VERSION`, such as `v0.9.0`. Pushing the tag triggers the release workflow.

## Safety Notes

- Avoid enabling `sdk.ShellTool` in production without sandboxing.
- Always configure `sdk.FilesystemTool` with a restricted root directory.
- Do not hard-code API keys in source code.
- Use separate `Session` values for independent conversations; create separate `Agent` instances when you need parallel runs or isolated tool state.

## License

Enno is released under the [MIT License](LICENSE).
