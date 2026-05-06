package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// baseURL 和 model 是当前网关的 OpenAI 兼容配置。
// 这个网关目前支持 /v2/chat/completions，所以这里仍然使用 Chat Completions。
const (
	baseURL = "https://maas-coding-api.cn-huabei-1.xf-yun.com/v2"
	model   = "astron-code-latest"
)

var (
	// workdir 是 agent 的工作目录，也是所有文件工具允许访问的根目录。
	workdir      = mustGetwd()
	systemPrompt = fmt.Sprintf(`You are a coding agent at %s.
Use the todo tool to plan multi-step tasks. Mark in_progress before starting, completed when done.
Prefer tools over prose.`, workdir)

	// todo 保存模型通过 todo 工具写入的结构化任务状态。
	// 这份状态只存在于本进程内，作用类似 Python 示例中的全局 TODO。
	todo = &todoManager{}

	// toolSpecs 是整个工具系统的声明表。
	// 每个工具在这里同时声明模型可见的 JSON Schema 和本地执行 handler。
	toolSpecs = []toolSpec{
		newTool("bash", "Run a shell command.", map[string]any{
			"command": map[string]any{"type": "string"},
		}, []string{"command"}, handleArgs("bash", func(args bashArgs) string {
			// bash 的 command 不能为空，否则 shell 会执行一个无意义的空命令。
			if strings.TrimSpace(args.Command) == "" {
				return "Error: missing command"
			}
			return runBash(args.Command)
		})),
		newTool("read_file", "Read file contents.", map[string]any{
			"path":  map[string]any{"type": "string"},
			"limit": map[string]any{"type": "integer"},
		}, []string{"path"}, handleArgs("read_file", func(args readFileArgs) string {
			return runRead(args.Path, args.Limit)
		})),
		newTool("write_file", "Write content to file.", map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		}, []string{"path", "content"}, handleArgs("write_file", func(args writeFileArgs) string {
			return runWrite(args.Path, args.Content)
		})),
		newTool("edit_file", "Replace exact text in file.", map[string]any{
			"path":     map[string]any{"type": "string"},
			"old_text": map[string]any{"type": "string"},
			"new_text": map[string]any{"type": "string"},
		}, []string{"path", "old_text", "new_text"}, handleArgs("edit_file", func(args editFileArgs) string {
			return runEdit(args.Path, args.OldText, args.NewText)
		})),
		newTool("todo", "Update task list. Track progress on multi-step tasks.", map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":     map[string]any{"type": "string"},
						"text":   map[string]any{"type": "string"},
						"status": map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
					},
					"required": []string{"id", "text", "status"},
				},
			},
		}, []string{"items"}, handleArgs("todo", func(args todoArgs) string {
			if err := todo.update(args.Items); err != nil {
				return fmt.Sprintf("Error: %v", err)
			}
			return todo.render()
		})),
	}

	// tools 是传给 OpenAI SDK 的工具定义；toolHandlers 是运行时按工具名查找 handler 的分发表。
	tools        = buildTools(toolSpecs)
	toolHandlers = buildToolHandlers(toolSpecs)
)

// toolHandler 是本地工具执行函数的统一签名。
// 参数保持为 RawMessage，让每个工具可以反序列化成自己的参数结构。
type toolHandler func(json.RawMessage) string

// toolSpec 把“模型看到的工具描述”和“本地执行逻辑”放在同一个结构里。
// 这样新增工具只需要新增一条 toolSpec，不用同时维护 schema 和 switch 分支。
type toolSpec struct {
	name        string
	description string
	properties  map[string]any
	required    []string
	handler     toolHandler
}

// bashArgs 对应 bash 工具的 JSON 参数。
type bashArgs struct {
	Command string `json:"command"`
}

// readFileArgs 对应 read_file 工具的 JSON 参数。
// Limit 为 0 时表示不限制读取行数。
type readFileArgs struct {
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

// writeFileArgs 对应 write_file 工具的 JSON 参数。
type writeFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// editFileArgs 对应 edit_file 工具的 JSON 参数。
type editFileArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

// todoArgs 对应 todo 工具的 JSON 参数。
type todoArgs struct {
	Items []todoItem `json:"items"`
}

// todoItem 是模型维护的单个任务项。
type todoItem struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
}

// todoManager 保存并校验任务列表。
type todoManager struct {
	items []todoItem
}

// update 校验并替换当前 todo 列表。
// 约束与 Python 版保持一致：最多 20 个任务，且同时只能有一个 in_progress。
func (m *todoManager) update(items []todoItem) error {
	if len(items) > 20 {
		return fmt.Errorf("max 20 todos allowed")
	}

	validated := make([]todoItem, 0, len(items))
	inProgressCount := 0
	for i, item := range items {
		item.ID = strings.TrimSpace(item.ID)
		item.Text = strings.TrimSpace(item.Text)
		item.Status = strings.ToLower(strings.TrimSpace(item.Status))
		if item.ID == "" {
			item.ID = fmt.Sprintf("%d", i+1)
		}
		if item.Text == "" {
			return fmt.Errorf("item %s: text required", item.ID)
		}
		switch item.Status {
		case "pending", "in_progress", "completed":
		default:
			return fmt.Errorf("item %s: invalid status %q", item.ID, item.Status)
		}
		if item.Status == "in_progress" {
			inProgressCount++
		}
		validated = append(validated, item)
	}
	if inProgressCount > 1 {
		return fmt.Errorf("only one task can be in_progress at a time")
	}

	m.items = validated
	return nil
}

// render 把 todo 列表渲染成适合终端和模型阅读的文本。
func (m *todoManager) render() string {
	if len(m.items) == 0 {
		return "No todos."
	}

	var lines []string
	completed := 0
	for _, item := range m.items {
		marker := map[string]string{
			"pending":     "[ ]",
			"in_progress": "[>]",
			"completed":   "[x]",
		}[item.Status]
		if item.Status == "completed" {
			completed++
		}
		lines = append(lines, fmt.Sprintf("%s #%s: %s", marker, item.ID, item.Text))
	}
	lines = append(lines, fmt.Sprintf("\n(%d/%d completed)", completed, len(m.items)))
	return strings.Join(lines, "\n")
}

// newTool 创建一个工具声明，保留和 Python 版本 TOOLS + TOOL_HANDLERS 类似的紧凑写法。
func newTool(name, description string, properties map[string]any, required []string, handler toolHandler) toolSpec {
	return toolSpec{
		name:        name,
		description: description,
		properties:  properties,
		required:    required,
		handler:     handler,
	}
}

// buildTools 把内部 toolSpec 转换为 OpenAI Go SDK 需要的工具数组。
func buildTools(specs []toolSpec) []openai.ChatCompletionToolUnionParam {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(specs))
	for _, spec := range specs {
		result = append(result, spec.openAITool())
	}
	return result
}

// buildToolHandlers 生成工具名到 handler 的分发表，用来执行模型发起的 tool call。
func buildToolHandlers(specs []toolSpec) map[string]toolHandler {
	result := make(map[string]toolHandler, len(specs))
	for _, spec := range specs {
		result[spec.name] = spec.handler
	}
	return result
}

// openAITool 把单个 toolSpec 翻译成 Chat Completions function tool。
func (spec toolSpec) openAITool() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        spec.name,
		Description: openai.String(spec.description),
		Parameters: shared.FunctionParameters{
			"type":                 "object",
			"properties":           spec.properties,
			"required":             spec.required,
			"additionalProperties": false,
		},
	})
}

// handleArgs 用泛型统一完成 JSON 参数解析，避免每个工具 handler 都重复 json.Unmarshal。
func handleArgs[T any](toolName string, handler func(T) string) toolHandler {
	return func(raw json.RawMessage) string {
		var args T
		if err := json.Unmarshal(raw, &args); err != nil {
			// 模型生成的工具参数不一定永远是合法 JSON，这里把解析错误作为工具结果返回给模型。
			return fmt.Sprintf("Error: invalid %s arguments: %v", toolName, err)
		}
		return handler(args)
	}
}

// main 负责初始化客户端并启动交互式 REPL。
// 用户每输入一轮问题，就把消息追加到 history，然后交给 agentLoop 处理工具调用闭环。
func main() {
	ctx := context.Background()

	// 这里使用 OpenAI Go SDK，但 baseURL 指向当前项目使用的讯飞 MaaS 兼容网关。
	client := openai.NewClient(
		option.WithAPIKey(apiKey()),
		option.WithBaseURL(baseURL),
	)

	// history 保存对话历史；每一轮 assistant/tool 结果都会追加进去，供后续轮次继续上下文。
	var history []openai.ChatCompletionMessageParamUnion
	reader := bufio.NewReader(os.Stdin)
	interactive := isTerminal(os.Stdin)

	for {
		// s03 的提示符与 Python 示例保持一致。
		// 只有连接到真实终端时才打印提示符；管道输入结束时不会留下半截 prompt。
		if interactive {
			fmt.Print("\033[36ms03 >> \033[0m")
		}
		query, err := reader.ReadString('\n')
		if err != nil && len(query) == 0 {
			if interactive {
				fmt.Println()
			}
			break
		}

		query = strings.TrimSpace(query)
		// 空输入、q、exit 都表示退出 REPL。
		if query == "" || strings.EqualFold(query, "q") || strings.EqualFold(query, "exit") {
			break
		}

		// 用户输入作为 user message 进入历史，然后 agentLoop 可能会多次请求模型和执行工具。
		history = append(history, openai.UserMessage(query))
		answer, err := agentLoop(ctx, client, &history)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			continue
		}
		if answer != "" {
			fmt.Println(answer)
		}
		fmt.Println()
	}
}

// agentLoop 是整个 agent 的核心循环。
// 它不断调用模型；如果模型返回 tool calls，就执行工具并把 tool result 回填；
// 如果模型没有继续调用工具，就返回最终文本回答。
func agentLoop(ctx context.Context, client openai.Client, history *[]openai.ChatCompletionMessageParamUnion) (string, error) {
	// roundsSinceTodo 记录本轮 agent loop 内连续多少次工具调用回合没有更新 todo。
	// 如果模型长时间忘记维护任务列表，就注入一个提醒消息。
	roundsSinceTodo := 0
	for {
		// system prompt 每次请求都放在最前面，但不写入 history，避免历史里重复堆叠 system 消息。
		messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(*history)+1)
		messages = append(messages, openai.SystemMessage(systemPrompt))
		messages = append(messages, (*history)...)

		// 把当前对话历史和工具定义一起发给模型，让模型自行决定是否调用工具。
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    model,
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("empty choices")
		}

		choice := resp.Choices[0]

		// assistant 消息必须进入 history。即使它只包含 tool calls，也需要回传给下一次请求。
		*history = append(*history, choice.Message.ToParam())
		if len(choice.Message.ToolCalls) == 0 {
			// 没有 tool calls 说明模型完成了本轮任务。
			return choice.Message.Content, nil
		}

		// 逐个执行模型请求的工具，并把结果以 tool message 追加回 history。
		usedTodo := false
		for _, toolCall := range choice.Message.ToolCalls {
			name, result := executeToolCall(toolCall)
			if name == "todo" {
				usedTodo = true
			}
			*history = append(*history, openai.ToolMessage(result, toolCall.ID))
		}
		if usedTodo {
			roundsSinceTodo = 0
		} else {
			roundsSinceTodo++
		}
		if roundsSinceTodo >= 3 {
			// Chat Completions 没有 Anthropic 那种同一 user message 内混排 tool_result 和 text block 的结构；
			// 这里在所有 tool messages 后追加一条普通 user reminder，表达同样的 nag 行为。
			*history = append(*history, openai.UserMessage("<reminder>Update your todos.</reminder>"))
		}
	}
}

// executeToolCall 根据模型返回的工具名查找本地 handler 并执行。
// 这里故意只做“分发”和“日志打印”，具体工具逻辑放在 runBash/runRead/runWrite/runEdit 中。
func executeToolCall(toolCall openai.ChatCompletionMessageToolCallUnion) (string, string) {
	functionCall := toolCall.AsFunction()
	name := functionCall.Function.Name

	// Chat Completions 会把 function arguments 放在字符串字段里，这里保持为 RawMessage 交给 handler 解析。
	handler, ok := toolHandlers[name]
	output := ""
	if ok {
		output = handler(json.RawMessage(functionCall.Function.Arguments))
	} else {
		output = fmt.Sprintf("Unknown tool: %s", name)
	}

	// 打印一小段工具输出，方便人在终端里观察 agent 正在做什么。
	fmt.Printf("> %s:\n", name)
	fmt.Println(truncate(output, 200))
	return name, output
}

// runBash 在工作目录内执行 shell 命令，并返回 stdout/stderr 合并后的结果。
func runBash(command string) string {
	// 这只是演示级防护：阻止几个明显危险的命令片段，生产环境需要更严格的沙箱和权限控制。
	dangerous := []string{"rm -rf /", "sudo", "shutdown", "reboot", "> /dev/"}
	for _, pattern := range dangerous {
		if strings.Contains(command, pattern) {
			return "Error: Dangerous command blocked"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 使用 sh -c 保持和 Python subprocess.run(shell=True) 类似的行为。
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workdir

	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))
	if ctx.Err() == context.DeadlineExceeded {
		return "Error: Timeout (120s)"
	}
	// 命令非零退出时仍把已有输出返回给模型，便于模型根据错误继续修正。
	if err != nil {
		if output == "" {
			return fmt.Sprintf("Error: %v", err)
		}
		output = output + "\n" + err.Error()
	}
	if output == "" {
		return "(no output)"
	}

	return truncate(output, 50000)
}

// runRead 读取 workspace 内文件内容，并支持按行数 limit 截断。
func runRead(path string, limit int) string {
	fp, err := safePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	bytes, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	lines := strings.Split(string(bytes), "\n")
	if limit > 0 && limit < len(lines) {
		// limit 只控制行数，最终仍会再经过总字符数截断，避免一次工具结果过大。
		remaining := len(lines) - limit
		lines = append(lines[:limit], fmt.Sprintf("... (%d more lines)", remaining))
	}
	return truncate(strings.Join(lines, "\n"), 50000)
}

// runWrite 写入 workspace 内文件；父目录不存在时会自动创建。
func runWrite(path, content string) string {
	fp, err := safePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path)
}

// runEdit 在 workspace 内文件中替换第一次出现的 oldText。
// 只替换一次是为了降低误改多个相同片段的风险。
func runEdit(path, oldText, newText string) string {
	fp, err := safePath(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	bytes, err := os.ReadFile(fp)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	content := string(bytes)
	if !strings.Contains(content, oldText) {
		return fmt.Sprintf("Error: Text not found in %s", path)
	}

	content = strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Edited %s", path)
}

// safePath 把模型传入的路径限制在 workdir 内。
// 这样 read/write/edit 工具不能通过 ../ 或绝对路径逃出当前项目目录。
func safePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("missing path")
	}

	var target string
	if filepath.IsAbs(path) {
		// 允许绝对路径输入，但仍要求它位于 workdir 之下。
		target = filepath.Clean(path)
	} else {
		target = filepath.Join(workdir, path)
	}

	// filepath.Rel 可以判断 target 是否仍在 workdir 内。
	rel, err := filepath.Rel(workdir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes workspace: %s", path)
	}
	return target, nil
}

// apiKey 优先读取 ENNO_API_KEY，方便本地覆盖。
// 如果没有设置环境变量，则使用当前示例中的 fallback key。
func apiKey() string {
	if key := os.Getenv("ENNO_API_KEY"); key != "" {
		return key
	}
	return "773f1ac8cd80a54f5bffe8450a7494de:NzhlOWM2NTA4MGZjZmY5NWRhZGIxYzEx"
}

// mustGetwd 获取当前工作目录；失败时用 "." 兜底，避免初始化阶段直接崩溃。
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// isTerminal 粗略判断输入是否来自交互式终端。
// 这样通过 printf/管道测试时不会打印 REPL prompt。
func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// truncate 按 rune 截断字符串，避免把中文字符截坏。
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
