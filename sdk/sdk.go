package sdk

import (
	"context"
	"fmt"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/builtintools/compact"
	"github.com/dean2021/enno/internal/builtintools/fetchurl"
	"github.com/dean2021/enno/internal/builtintools/filesystem"
	"github.com/dean2021/enno/internal/builtintools/glob"
	"github.com/dean2021/enno/internal/builtintools/grep"
	"github.com/dean2021/enno/internal/builtintools/loadskill"
	"github.com/dean2021/enno/internal/builtintools/shell"
	"github.com/dean2021/enno/internal/builtintools/subagent"
	"github.com/dean2021/enno/internal/builtintools/taskgraph"
	"github.com/dean2021/enno/internal/systemprompt"
)

type Config struct {
	Provider      enno.Provider
	SystemPrompt  string
	BuiltinTools  BuiltinTools
	Permissions   ToolPermissions
	CustomTools   []enno.Tool
	Options       enno.RequestOptions
	Hooks         []enno.Hook
	Policies      []enno.Policy
	EventHandler  enno.EventHandler
	Compaction    *enno.CompactionConfig
	MaxToolRounds int
}

type BuiltinTools struct {
	TaskGraph  *TaskGraphTool
	Filesystem *FilesystemTool
	Shell      *ShellTool
	Grep       *GrepTool
	Glob       *GlobTool
	FetchURL   *FetchURLTool
	Subagent   *SubagentTool
	LoadSkill  *LoadSkillTool
	Compact    *CompactTool
}

type TaskGraphTool struct {
	Root     string
	TasksDir string
	Timeout  time.Duration
}

type FilesystemTool struct {
	Root           string
	Read           bool
	Write          bool
	MaxOutputChars int
}

type ShellTool struct {
	Workdir        string
	Timeout        time.Duration
	DenyList       []string
	MaxOutputChars int
	SafetyPolicy   ShellSafetyPolicy
}

type ShellSafetyPolicy = shell.SafetyPolicy

const (
	ShellSafetyPolicyDenyList = shell.SafetyPolicyDenyList
	ShellSafetyPolicyAllowAll = shell.SafetyPolicyAllowAll
)

type GrepTool struct {
	Root           string
	Timeout        time.Duration
	MaxOutputChars int
}

type GlobTool struct {
	Root           string
	Timeout        time.Duration
	MaxOutputChars int
}

type FetchURLTool struct {
	Timeout        time.Duration
	MaxOutputChars int
	UserAgent      string
}

type SubagentTool struct {
	SystemPrompt   string
	MaxToolRounds  int
	MaxResultChars int
	ToolName       string
	EventHandler   enno.EventHandler
}

type LoadSkillTool struct {
	Dirs []string
}

type CompactTool struct{}

type assembledBuiltins struct {
	Tools         []enno.Tool
	SkillsSummary string
}

func NewAgent(config Config) (*enno.Agent, error) {
	agentConfig, err := AssembleConfig(config)
	if err != nil {
		return nil, err
	}
	return enno.NewAgent(agentConfig)
}

func AssembleConfig(config Config) (enno.Config, error) {
	builtins, err := buildChildTools(config.BuiltinTools)
	if err != nil {
		return enno.Config{}, err
	}
	var permission *permissionHook
	if !config.Permissions.IsZero() {
		permissions, err := config.Permissions.withDefaults()
		if err != nil {
			return enno.Config{}, err
		}
		permission = &permissionHook{permissions: permissions}
	}

	childTools := builtins.Tools
	tools := append([]enno.Tool(nil), childTools...)

	if shouldRegisterCompact(config) {
		tools = append(tools, compact.New())
	}
	if config.BuiltinTools.Subagent != nil {
		if len(childTools) == 0 {
			return enno.Config{}, fmt.Errorf("sdk: subagent enabled but no child tools configured")
		}
		subagentTool, err := subagent.New(subagent.Config{
			Provider:       config.Provider,
			ChildTools:     append([]enno.Tool(nil), childTools...),
			SystemPrompt:   config.BuiltinTools.Subagent.SystemPrompt,
			MaxToolRounds:  config.BuiltinTools.Subagent.MaxToolRounds,
			MaxResultChars: config.BuiltinTools.Subagent.MaxResultChars,
			ToolName:       config.BuiltinTools.Subagent.ToolName,
			EventHandler:   config.BuiltinTools.Subagent.EventHandler,
			Hooks:          childPermissionHooks(permission),
		})
		if err != nil {
			return enno.Config{}, err
		}
		tools = append(tools, subagentTool)
	}
	tools = append(tools, config.CustomTools...)

	hooks := append([]enno.Hook(nil), config.Hooks...)
	if permission != nil {
		hooks = append([]enno.Hook{*permission}, hooks...)
	}

	return enno.Config{
		Provider:      config.Provider,
		SystemPrompt:  systemprompt.Join(config.SystemPrompt, []systemprompt.Section{systemprompt.SkillsSection(builtins.SkillsSummary)}),
		Tools:         tools,
		MaxToolRounds: config.MaxToolRounds,
		EventHandler:  config.EventHandler,
		Compaction:    config.Compaction,
		Options:       config.Options,
		Policies:      append([]enno.Policy(nil), config.Policies...),
		Hooks:         hooks,
	}, nil
}

func childPermissionHooks(permission *permissionHook) []enno.Hook {
	if permission == nil {
		return nil
	}
	return []enno.Hook{*permission}
}

func buildChildTools(config BuiltinTools) (assembledBuiltins, error) {
	var tools []enno.Tool
	var skillsSummary string
	if config.TaskGraph != nil {
		tools = append(tools, taskgraph.New(taskgraph.Config{
			Root:     config.TaskGraph.Root,
			TasksDir: config.TaskGraph.TasksDir,
			Timeout:  config.TaskGraph.Timeout,
		})...)
	}
	if config.Filesystem != nil {
		fsTools := filesystem.New(filesystem.Config{
			Root:           config.Filesystem.Root,
			MaxOutputChars: config.Filesystem.MaxOutputChars,
		})
		tools = append(tools, filterFilesystemTools(fsTools, config.Filesystem)...)
	}
	if config.Shell != nil {
		tools = append(tools, shell.New(shell.Config{
			Workdir:        config.Shell.Workdir,
			Timeout:        config.Shell.Timeout,
			DenyList:       append([]string(nil), config.Shell.DenyList...),
			MaxOutputChars: config.Shell.MaxOutputChars,
			SafetyPolicy:   config.Shell.SafetyPolicy,
		}))
	}
	if config.Grep != nil {
		tools = append(tools, grep.New(grep.Config{
			Root:           config.Grep.Root,
			Timeout:        config.Grep.Timeout,
			MaxOutputChars: config.Grep.MaxOutputChars,
		}))
	}
	if config.Glob != nil {
		tools = append(tools, glob.New(glob.Config{
			Root:           config.Glob.Root,
			Timeout:        config.Glob.Timeout,
			MaxOutputChars: config.Glob.MaxOutputChars,
		}))
	}
	if config.FetchURL != nil {
		tools = append(tools, fetchurl.New(fetchurl.Config{
			Timeout:        config.FetchURL.Timeout,
			MaxOutputChars: config.FetchURL.MaxOutputChars,
			UserAgent:      config.FetchURL.UserAgent,
		}))
	}
	if config.LoadSkill != nil && len(config.LoadSkill.Dirs) > 0 {
		registry, err := loadskill.LoadDirs(config.LoadSkill.Dirs)
		if err != nil {
			return assembledBuiltins{}, err
		}
		if registry.Count() > 0 {
			tool, err := loadskill.NewTool(registry)
			if err != nil {
				return assembledBuiltins{}, err
			}
			tools = append(tools, tool)
			skillsSummary = registry.DescriptionsText()
		}
	}
	return assembledBuiltins{Tools: tools, SkillsSummary: skillsSummary}, nil
}

func filterFilesystemTools(tools []enno.Tool, config *FilesystemTool) []enno.Tool {
	readEnabled := config.Read || (!config.Read && !config.Write)
	writeEnabled := config.Write || (!config.Read && !config.Write)
	filtered := make([]enno.Tool, 0, len(tools))
	for _, tool := range tools {
		switch tool.Name {
		case "read_file":
			if readEnabled {
				filtered = append(filtered, tool)
			}
		case "write_file", "edit_file":
			if writeEnabled {
				filtered = append(filtered, tool)
			}
		default:
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func shouldRegisterCompact(config Config) bool {
	if config.BuiltinTools.Compact != nil {
		return true
	}
	return config.Compaction != nil && config.Compaction.Enabled
}

type PermissionMode string

const (
	PermissionAsk   PermissionMode = "ask"
	PermissionAllow PermissionMode = "allow"
	PermissionDeny  PermissionMode = "deny"
)

type ToolPermissions struct {
	Mode            PermissionMode
	AllowedTools    []string
	DisallowedTools []string
}

func (p ToolPermissions) IsZero() bool {
	return p.Mode == "" && len(p.AllowedTools) == 0 && len(p.DisallowedTools) == 0
}

func (p ToolPermissions) withDefaults() (ToolPermissions, error) {
	if p.Mode == "" {
		p.Mode = PermissionAllow
	}
	switch p.Mode {
	case PermissionAsk, PermissionAllow, PermissionDeny:
	default:
		return ToolPermissions{}, fmt.Errorf("sdk: invalid permission mode %q", p.Mode)
	}
	p.AllowedTools = append([]string(nil), p.AllowedTools...)
	p.DisallowedTools = append([]string(nil), p.DisallowedTools...)
	return p, nil
}

type permissionHook struct {
	enno.NoopHook
	permissions ToolPermissions
}

func (h permissionHook) BeforeToolCall(ctx context.Context, state enno.BeforeToolCallState) (enno.BeforeToolCallResult, error) {
	name := state.ToolCall.Name
	if containsTool(h.permissions.DisallowedTools, name) {
		return denyTool(name), nil
	}
	if len(h.permissions.AllowedTools) > 0 && !containsTool(h.permissions.AllowedTools, name) {
		return denyTool(name), nil
	}
	if h.permissions.Mode == PermissionDeny && len(h.permissions.AllowedTools) == 0 {
		return denyTool(name), nil
	}
	return enno.BeforeToolCallResult{}, nil
}

func denyTool(name string) enno.BeforeToolCallResult {
	return enno.BeforeToolCallResult{
		Deny:        true,
		DenyMessage: fmt.Sprintf("Error: tool %s denied by permissions", name),
	}
}

func containsTool(tools []string, name string) bool {
	for _, tool := range tools {
		if tool == name {
			return true
		}
	}
	return false
}
