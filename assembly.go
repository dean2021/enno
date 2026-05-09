package enno

import (
	"context"
	"fmt"

	"github.com/dean2021/enno/prompt"
)

// ToolBuilder resolves a BuiltinTools config into assembled Tool slices and a
// skills summary. The default builder is nil; call RegisterToolBuilder or
// import the enno/setup package to install one. This indirection avoids import
// cycles between the root package and builtintools/*.
var toolBuilder ToolBuilder

// ToolBuilder constructs Tool slices from BuiltinTools configuration.
type ToolBuilder interface {
	BuildTools(BuiltinTools) (BuiltTools, error)
	BuildCompact() ([]Tool, error)
	BuildSubagent(SubagentTool, Provider, []Tool, []Hook) (Tool, error)
}

// BuiltTools holds the result of assembling built-in tool configuration.
type BuiltTools struct {
	Tools         []Tool
	SkillsSummary string
}

// RegisterToolBuilder installs the tool builder used by NewAgent when
// BuiltinTools or related config fields are set. Call this from a wiring
// package (e.g. main) that imports both enno and the builtintools packages.
func RegisterToolBuilder(b ToolBuilder) {
	toolBuilder = b
}

// AssembleConfig resolves the high-level Config fields (BuiltinTools,
// SystemPromptSections, Permissions, CustomTools) into a low-level Config
// ready for Agent construction. It is called automatically by NewAgent when
// any assembly-level fields are set.
func AssembleConfig(config Config) (Config, error) {
	var skillsSummary string
	var childTools []Tool

	if hasBuiltinTools(config) {
		if toolBuilder == nil {
			return Config{}, fmt.Errorf("enno: BuiltinTools configured but no ToolBuilder registered; import the enno/setup package or call RegisterToolBuilder before NewAgent")
		}
		builtins, err := toolBuilder.BuildTools(config.BuiltinTools)
		if err != nil {
			return Config{}, err
		}
		childTools = builtins.Tools
		skillsSummary = builtins.SkillsSummary
	}

	var permission *permissionHook
	if !config.Permissions.IsZero() {
		permissions, err := config.Permissions.withDefaults()
		if err != nil {
			return Config{}, err
		}
		permission = &permissionHook{permissions: permissions}
	}

	tools := append([]Tool(nil), config.Tools...)
	tools = append(tools, childTools...)

	if shouldRegisterCompact(config) {
		if toolBuilder == nil {
			return Config{}, fmt.Errorf("enno: Compact tool requested but no ToolBuilder registered; import the enno/setup package or call RegisterToolBuilder before NewAgent")
		}
		compactTools, err := toolBuilder.BuildCompact()
		if err != nil {
			return Config{}, err
		}
		tools = append(tools, compactTools...)
	}

	if config.BuiltinTools.Subagent != nil {
		if toolBuilder == nil {
			return Config{}, fmt.Errorf("enno: Subagent configured but no ToolBuilder registered; import the enno/setup package or call RegisterToolBuilder before NewAgent")
		}
		if len(childTools) == 0 {
			return Config{}, fmt.Errorf("enno: subagent enabled but no child tools configured")
		}
		subagentHooks := childPermissionHooks(permission)
		subagentTool, err := toolBuilder.BuildSubagent(*config.BuiltinTools.Subagent, config.Provider, append([]Tool(nil), childTools...), subagentHooks)
		if err != nil {
			return Config{}, err
		}
		tools = append(tools, subagentTool)
	}
	tools = append(tools, config.CustomTools...)

	hooks := append([]Hook(nil), config.Hooks...)
	if permission != nil {
		hooks = append([]Hook{*permission}, hooks...)
	}

	return Config{
		Provider:      config.Provider,
		SystemPrompt:  assembleSystemPrompt(config.SystemPrompt, config.SystemPromptSections, skillsSummary),
		Tools:         tools,
		MaxToolRounds: config.MaxToolRounds,
		EventHandler:  config.EventHandler,
		Compaction:    config.Compaction,
		Options:       config.Options,
		Policies:      append([]Policy(nil), config.Policies...),
		Hooks:         hooks,
	}, nil
}

func assembleSystemPrompt(base string, customSections []SystemPromptSection, skillsSummary string) string {
	sections := make([]prompt.Section, 0, len(customSections)+1)
	for _, section := range customSections {
		sections = append(sections, prompt.Section{
			Name:    section.Name,
			Content: section.Content,
		})
	}
	sections = append(sections, prompt.RuntimeSections(prompt.RuntimeConfig{
		SkillsSummary: skillsSummary,
	})...)
	return prompt.Join(base, sections)
}

func childPermissionHooks(permission *permissionHook) []Hook {
	if permission == nil {
		return nil
	}
	return []Hook{*permission}
}

func hasBuiltinTools(config Config) bool {
	bt := config.BuiltinTools
	return bt.TaskGraph != nil || bt.Filesystem != nil || bt.Shell != nil ||
		bt.Grep != nil || bt.Glob != nil || bt.FetchURL != nil ||
		bt.Subagent != nil || bt.LoadSkill != nil
}

func shouldRegisterCompact(config Config) bool {
	return config.BuiltinTools.Compact != nil
}

type permissionHook struct {
	NoopHook
	permissions ToolPermissions
}

func (h permissionHook) BeforeToolCall(ctx context.Context, state BeforeToolCallState) (BeforeToolCallResult, error) {
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
	return BeforeToolCallResult{}, nil
}

func denyTool(name string) BeforeToolCallResult {
	return BeforeToolCallResult{
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
