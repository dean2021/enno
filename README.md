# Enno

Enno is a lightweight, provider-neutral Go agent framework. It owns the core
Agent loop, explicit session state, tool dispatch, provider interfaces, SDK
assembly helpers, and built-in tool implementations.

The former in-repo CLI has moved to the standalone Godo project at
`../godo-coding-agent`. Enno now focuses
on SDK/provider functionality; applications and CLIs should depend on Enno
through public packages only.

Repository: [github.com/dean2021/enno](https://github.com/dean2021/enno)

## Features

- Provider-neutral core package: `Agent`, `Session`, `RunResult`, `Provider`,
  `Tool`, `Message`, `Request`, and `Response`.
- OpenAI-compatible provider via `provider/openai`.
- Anthropic Messages API provider via `provider/anthropic`.
- High-level `sdk` package for configuring built-in tools, custom tools,
  permissions, hooks, policies, compaction, and prompt sections.
- Built-in tools for task graph, filesystem, shell, grep, glob, fetch_url,
  subagent, load_skill, and compact.
- Optional events for observing model calls, tool calls, results, and token
  usage without exposing hidden chain-of-thought.

## Installation

```sh
go get github.com/dean2021/enno
```

For the standalone coding-agent CLI, use the Godo project:

```sh
cd ../godo-coding-agent
go install ./cmd/godo
```

## SDK Quick Start

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
  provider/internal     provider-shared helpers
  sdk                   high-level SDK assembler and built-in tool config
  internal/builtintools internal built-in tool implementations
  internal/systemprompt SDK runtime prompt helpers
  examples              SDK usage examples
  docs                  design, usage, release, and migration docs
```

See:

- [Design document](docs/design.md)
- [SDK usage guide](docs/usage-sdk.md)
- [CLI migration note](docs/usage-cli.md)
- [Release guide](docs/release.md)
- [Migration guide](docs/migration.md)
- [Changelog](CHANGELOG.md)

## Development

```sh
make help
make test
make verify
```

`make verify` formats code, tidies modules, and runs all SDK tests.

## Safety Notes

- Avoid enabling `sdk.ShellTool` in production without sandboxing.
- Always configure `sdk.FilesystemTool` with a restricted root directory.
- Do not hard-code API keys in source code.
- Use separate `Session` values for independent conversations.

## License

Enno is released under the [MIT License](LICENSE).
