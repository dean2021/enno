# CLI Migration Note

The Enno CLI has moved out of this SDK repository. The standalone coding-agent
CLI is now Godo:

```text
../godo-coding-agent
```

Install Godo locally from that repository:

```sh
cd ../godo-coding-agent
go install ./cmd/godo
```

Common commands:

```sh
godo
godo run "Analyze this repository"
godo --config /path/to/config.yaml
godo --workdir .
```

Godo owns CLI behavior including YAML config, TUI, history, project instruction
loading, coding-agent prompt assembly, and app-owned default paths. Enno remains
the SDK dependency used by Godo through public packages:

- `github.com/dean2021/enno`
- `github.com/dean2021/enno/sdk`
- `github.com/dean2021/enno/provider/openai`
- `github.com/dean2021/enno/provider/anthropic`

Former Enno CLI paths moved to Godo-owned paths:

| Old path | New path |
|---|---|
| `~/.enno/config.yaml` | `~/.godo/config.yaml` |
| `~/.enno/history.jsonl` | `~/.godo/history.jsonl` |
| `~/.enno/skills` | `~/.godo/skills` |
| `~/.enno/tasks/<session_id>/` | `~/.godo/tasks/<session_id>/` |
| `~/.enno/transcripts` | `~/.godo/transcripts` |

No automatic migration is performed. Copy any desired files manually and adjust
paths in `config.yaml`.
