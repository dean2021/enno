# Enno CLI 使用指南

本文档介绍 Enno CLI 的安装、配置与使用。SDK / Package 用法请参考 [SDK 使用指南](usage-sdk.md)。

## 安装

本地开发时在项目根目录安装：

```sh
go install ./cmd/enno
```

发布后通过模块路径安装：

```sh
go install github.com/dean2021/enno/cmd/enno@latest
```

## 交互模式

```sh
enno
```

进入基于 **bubbletea**（Charm）与 **lipgloss** 的 TUI 交互界面后输入任务并按 Enter 提交。

**界面布局：**

- 深色主题，风格贴近 **Claude Code** 的终端体验
- 用户消息显示为 **You** 标签与背景条，助手为 **Enno**
- 底部输入框与上方会话区以弱边框分隔
- 就绪提示与快捷键说明显示在输入框上方
- 主会话区是单一流：用户输入、模型进度、工具调用、工具参数、弱化显示的工具结果和最终回答按时间顺序追加

**快捷键：**

| 操作 | 快捷键 |
|---|---|
| 提交输入 | Enter |
| 退出 | Esc / Ctrl+C / q / exit |
| 切换焦点（输入 ↔ 会话区） | Tab |
| 浏览历史输入（输入区焦点时） | Alt+↑ / Alt+↓ |
| 滚动会话区（输入区焦点时） | Ctrl+↑ / Ctrl+↓ |
| 滚动会话区（会话区焦点时） | ↑ / ↓ / PgUp / PgDn |
| 跳到顶部 / 底部（会话区） | gg / G |
| 跳转搜索 | / 或 Ctrl+F |
| 展开工具结果 | 鼠标点击折叠行 |
| 鼠标滚轮滚动 | 始终滚动会话区（无论焦点） |

**关闭鼠标捕获：**

设置环境变量 `CLAUDE_CODE_DISABLE_MOUSE=1`（值为 `1` / `true` / `yes` / `on`）可关闭鼠标捕获，保留备用屏 TUI，改用键盘滚动。部分终端里划选需 Shift+拖拽。

界面不会展示模型隐藏思维链。

## 单次执行

```sh
enno run "帮我分析当前目录的 Go 包结构"
```

`run` 模式使用独立的显式 Session 执行一次 Agent 调用，输出 `RunResult.Content` 后退出，不进入交互界面。

## 配置文件

CLI 自动读取配置文件：

```text
~/.enno/config.yaml
```

默认配置文件不存在时，CLI 会自动创建一个带注释的模板文件，然后继续按现有配置解析。也可以显式指定配置文件：

```sh
enno --config /path/to/config.yaml
enno run --config /path/to/config.yaml "帮我分析当前目录"
```

Provider、model、api_key、base_url、max_tokens 等模型配置**只从 YAML 读取**，不读取环境变量，也不提供对应 flags。

### OpenAI 兼容配置

```yaml
provider: openai
model: your-model
api_key: your-key
base_url: https://example.com/v1
max_tokens: 4096
shell: true
filesystem: true
```

### Anthropic 配置

```yaml
provider: anthropic
model: claude-sonnet-4-5-20250929
api_key: your-anthropic-key
max_tokens: 4096
shell: false
filesystem: true
```

### 配置字段说明

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `provider` | string | `openai` | 模型供应商，支持 `openai` 或 `anthropic` |
| `model` | string | — | 模型名称，必填 |
| `api_key` | string | — | 供应商 API key |
| `base_url` | string | — | OpenAI 兼容 provider 必填 |
| `max_tokens` | int | `4096` | Anthropic 最大输出 token 数 |
| `http_proxy` | string | — | HTTP/SOCKS5 代理 URL（`http://`、`https://`、`socks5://`、`socks5h://`） |
| `proxy` | string | — | `http_proxy` 的别名键（同时存在时 `http_proxy` 优先） |
| `max_http_retries` | int | `6` | SDK HTTP 重试次数（429 / 5xx / 超时 / 连接错误） |
| `shell` | bool | `true` | 是否启用 shell 工具 |
| `filesystem` | bool | `true` | 是否启用文件系统工具 |
| `subagent` | bool | `false` | 是否启用子 Agent 工具 |
| `grep` | bool | `true` | 是否启用 grep 工具（需系统安装 `rg`） |
| `glob` | bool | `true` | 是否启用 glob 工具（需系统安装 `rg`） |
| `task_graph` | bool | `true` | 是否启用任务图工具 |
| `skills_dir` | string | — | 单个额外 skill 目录 |
| `skills_extra_dirs` | []string | — | 额外 skill 目录列表 |
| `compaction` | bool 或 mapping | — | 上下文压缩配置（见下文） |

### 代理配置

`http_proxy` 支持：

- HTTP 代理：`http://127.0.0.1:7890`
- HTTPS 代理：`https://127.0.0.1:7890`
- SOCKS5 代理：`socks5://127.0.0.1:7891`

不设置时走 SDK 默认客户端（仍可使用环境变量 `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY`）。

### HTTP 重试

底层 SDK（OpenAI / Anthropic 官方 Go 客户端）已对 429、5xx、请求超时、连接错误做退避重试，并识别 `Retry-After`。Enno 默认 `MaxRetries` 设为 6（共 7 次 HTTP 尝试），高于 SDK 自带的 2，以减少网关偶发 500 导致的失败；设为正整数可覆盖该默认值。

### 任务图

CLI 下任务数据写入 `~/.enno/tasks/<session_id>/`（`session_id` 为本次进程 UUID v4，不随 `--workdir` 变化；`grep` / `bash` 等仍受 `--workdir` 约束）。

### 技能目录

CLI 按以下顺序合并 skill 目录（后者覆盖同名 skill）：

1. `~/.enno/skills`（若路径不存在则跳过）
2. `skills_extra_dirs` 列表中的每项
3. `skills_dir` 单个目录
4. `--skills-dir` flag

路径支持 `~`，缺失目录跳过不报错。合并后若至少解析到一个 skill，则注册 `load_skill` 并在 system prompt 中追加摘要。

### 上下文压缩

CLI 自动创建的模板文件默认带有 `compaction.enabled: true`（可随时改为 `false` 关闭）。启用后会注册 `compact` 工具、执行 micro 压缩；仅在估算超过阈值或模型调用 `compact` 时才会多一次摘要模型调用（计费），并在 `transcript_dir` 写入 JSONL。

简写形式：

```yaml
compaction: true
```

映射形式：

```yaml
compaction:
  enabled: true
  transcript_dir: ~/.enno/transcripts
  model_context_tokens: 200000
  auto_compact_buffer_tokens: 13000
  auto_compact_input_tokens: 50000
  keep_recent_tool_results: 3
  micro_compact_min_chars: 100
  micro_compact_tool_names:
    - bash
    - read_file
  skip_on_summarize_error: false
```

| 字段 | 说明 |
|---|---|
| `model_context_tokens` | 若设置，自动压缩阈值优先为「该值 − `auto_compact_buffer_tokens`」 |
| `auto_compact_buffer_tokens` | buffer 默认 13000；仅 `model_context_tokens > 0` 时生效 |
| `auto_compact_input_tokens` | 默认 50000；`model_context_tokens` 为零时使用此值 |
| `keep_recent_tool_results` | micro 保留最近 N 条工具结果全文，默认 3 |
| `micro_compact_min_chars` | 超过此长度的工具结果才被替换为占位，默认 100 |
| `micro_compact_tool_names` | 仅对这些工具名触发 micro；空 = 所有工具结果参与 |
| `skip_on_summarize_error` | 自动摘要失败时跳过而非中断；手动 `compact` 仍报错 |
| `transcript_dir` | JSONL 存档目录，支持 `~`；默认 `~/.enno/transcripts` |

## 常用 Flags

```sh
enno --workdir .
enno --no-shell
enno --no-filesystem
enno --no-grep
enno --no-glob
enno --no-task-graph
enno --no-subagent
enno --skills-dir /path/to/more-skills
```

| Flag | 说明 |
|---|---|
| `--workdir` | 工具工作目录，默认当前目录 |
| `--no-shell` | 关闭 shell 工具 |
| `--no-filesystem` | 关闭文件系统工具 |
| `--no-grep` | 关闭 grep 工具 |
| `--no-glob` | 关闭 glob 工具 |
| `--no-task-graph` | 关闭任务图工具 |
| `--no-subagent` | 关闭子 Agent 工具 |
| `--skills-dir` | 追加额外 skill 目录（合并顺序在配置之后） |
| `--config` | 指定配置文件路径 |
| `--prompt` | REPL 提示符，默认 `enno >>` |

`--no-*` flags 与配置文件中的对应字段冲突时，以关闭为准。例如配置中 `grep: true` + 命令行 `--no-grep` → grep 关闭。

## 工具装配顺序

CLI 默认在子 Agent 与父 Agent 中装配同一套子工具，顺序为：

1. `task_*`（若开启）
2. 文件系统工具（若开启）
3. `bash`（若开启）
4. `grep`（若开启）
5. `glob`（若开启）
6. 其他（如 `load_skill` 等）
7. `compact`（需要时）
8. `subagent`（若开启，仅父级挂载；子 Agent 不含该工具）
