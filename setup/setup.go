// Package setup registers the default ToolBuilder that constructs built-in tools
// from enno.BuiltinTools configuration. Import this package in your main function
// (typically via a blank import) to enable built-in tool assembly:
//
//	import _ "github.com/dean2021/enno/setup"
//
// After this import, enno.NewAgent will be able to resolve BuiltinTools, Compact,
// Subagent, and other built-in tool configuration into concrete tool instances.
package setup

import (
	"github.com/dean2021/enno"
	"github.com/dean2021/enno/builtintools/compact"
	"github.com/dean2021/enno/builtintools/fetchurl"
	"github.com/dean2021/enno/builtintools/filesystem"
	"github.com/dean2021/enno/builtintools/glob"
	"github.com/dean2021/enno/builtintools/grep"
	loadskill2 "github.com/dean2021/enno/builtintools/loadskill"
	"github.com/dean2021/enno/builtintools/shell"
	"github.com/dean2021/enno/builtintools/subagent"
	"github.com/dean2021/enno/builtintools/taskgraph"
)

func init() {
	enno.RegisterToolBuilder(defaultBuilder{})
}

type defaultBuilder struct{}

func (defaultBuilder) BuildTools(bt enno.BuiltinTools) (enno.BuiltTools, error) {
	var tools []enno.Tool
	var skillsSummary string

	if bt.TaskGraph != nil {
		tools = append(tools, taskgraph.New(taskgraph.Config{
			Root:     bt.TaskGraph.Root,
			TasksDir: bt.TaskGraph.TasksDir,
			Timeout:  bt.TaskGraph.Timeout,
		})...)
	}
	if bt.Filesystem != nil {
		fsTools := filesystem.New(filesystem.Config{
			Root:           bt.Filesystem.Root,
			MaxOutputChars: bt.Filesystem.MaxOutputChars,
		})
		tools = append(tools, filterFilesystemTools(fsTools, bt.Filesystem)...)
	}
	if bt.Shell != nil {
		tools = append(tools, shell.New(shell.Config{
			Workdir:        bt.Shell.Workdir,
			Timeout:        bt.Shell.Timeout,
			DenyList:       append([]string(nil), bt.Shell.DenyList...),
			MaxOutputChars: bt.Shell.MaxOutputChars,
			SafetyPolicy:   shell.SafetyPolicy(bt.Shell.SafetyPolicy),
		}))
	}
	if bt.Grep != nil {
		tools = append(tools, grep.New(grep.Config{
			Root:           bt.Grep.Root,
			Timeout:        bt.Grep.Timeout,
			MaxOutputChars: bt.Grep.MaxOutputChars,
		}))
	}
	if bt.Glob != nil {
		tools = append(tools, glob.New(glob.Config{
			Root:           bt.Glob.Root,
			Timeout:        bt.Glob.Timeout,
			MaxOutputChars: bt.Glob.MaxOutputChars,
		}))
	}
	if bt.FetchURL != nil {
		tools = append(tools, fetchurl.New(fetchurl.Config{
			Timeout:        bt.FetchURL.Timeout,
			MaxOutputChars: bt.FetchURL.MaxOutputChars,
			UserAgent:      bt.FetchURL.UserAgent,
		}))
	}
	if bt.LoadSkill != nil && len(bt.LoadSkill.Dirs) > 0 {
		registry, err := loadskill2.LoadDirs(bt.LoadSkill.Dirs)
		if err != nil {
			return enno.BuiltTools{}, err
		}
		if registry.Count() > 0 {
			tool, err := loadskill2.NewTool(registry)
			if err != nil {
				return enno.BuiltTools{}, err
			}
			tools = append(tools, tool)
			skillsSummary = registry.DescriptionsText()
		}
	}

	return enno.BuiltTools{Tools: tools, SkillsSummary: skillsSummary}, nil
}

func (defaultBuilder) BuildCompact() ([]enno.Tool, error) {
	return []enno.Tool{compact.New()}, nil
}

func (defaultBuilder) BuildSubagent(st enno.SubagentTool, provider enno.Provider, childTools []enno.Tool, hooks []enno.Hook) (enno.Tool, error) {
	return subagent.New(subagent.Config{
		Provider:       provider,
		ChildTools:     childTools,
		SystemPrompt:   st.SystemPrompt,
		MaxToolRounds:  st.MaxToolRounds,
		MaxResultChars: st.MaxResultChars,
		ToolName:       st.ToolName,
		EventHandler:   st.EventHandler,
		Hooks:          hooks,
	})
}

func filterFilesystemTools(tools []enno.Tool, config *enno.FilesystemTool) []enno.Tool {
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
