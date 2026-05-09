package systemprompt

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuilderDefaultSections(t *testing.T) {
	env := Environment{
		Workdir:    "/repo",
		Date:       "2026-05-09",
		Platform:   "darwin/arm64",
		Shell:      "zsh",
		OSVersion:  "Darwin 25.3.0",
		IsGit:      true,
		IsGitKnown: true,
	}
	prompt := New(Config{
		Identity:  "You are a coding agent at /repo.\nPrefer tools over prose.",
		SessionID: "session-1",
		Tools: ToolSet{
			TaskGraph:  true,
			Filesystem: true,
			Shell:      true,
			Grep:       true,
			Glob:       true,
			FetchURL:   true,
			Subagent:   true,
		},
		Environment:       &env,
		CompactionEnabled: true,
	}).Build()

	for _, want := range []string{
		"# Identity",
		"You are a coding agent at /repo.",
		"# System",
		"All text you output outside of tool use is displayed to the user",
		"Context compaction may automatically summarize",
		"# Doing Tasks",
		"Do not propose changes to code you haven't read",
		"at most two materially different fixes",
		"Avoid unrelated refactors, speculative abstractions",
		"Verify meaningful code changes",
		"# Safety",
		"Do not generate or guess URLs",
		"prompt injection",
		"OWASP Top 10",
		"Carefully consider the reversibility and blast radius",
		"# Environment",
		"- Current date: 2026-05-09",
		"# Tool Guidance",
		"Use read_file, write_file, and edit_file for file operations",
		"Use task_create, task_update, task_list, and task_get",
		"~/.enno/tasks/session-1/",
		"Use fetch_url",
		"Context compaction is enabled",
		"prefer it over bash",
		"# Communication",
		"Be concise and direct",
		"file_path:line_number",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuilderOmitsDisabledToolGuidance(t *testing.T) {
	prompt := New(Config{
		Tools: ToolSet{Grep: true},
	}).Build()
	if !strings.Contains(prompt, "Use the grep tool") {
		t.Fatalf("expected grep guidance:\n%s", prompt)
	}
	for _, notWant := range []string{"Use the glob tool", "Use fetch_url", "task_create"} {
		if strings.Contains(prompt, notWant) {
			t.Fatalf("unexpected %q in prompt:\n%s", notWant, prompt)
		}
	}
}

func TestBuilderOmitsIdentityWhenUnset(t *testing.T) {
	prompt := New(Config{
		Tools: ToolSet{Grep: true},
	}).Build()
	if strings.Contains(prompt, "# Identity") || strings.Contains(prompt, "coding agent") {
		t.Fatalf("unexpected default identity:\n%s", prompt)
	}
}

func TestBuilderOmitsCompactionSystemTextWhenDisabled(t *testing.T) {
	prompt := New(Config{}).Build()
	if strings.Contains(prompt, "Context compaction may automatically summarize") {
		t.Fatalf("unexpected compaction system text:\n%s", prompt)
	}
}

func TestSkillsSection(t *testing.T) {
	section := SkillsSection("  - demo: test skill")
	got := section.String()
	if !strings.Contains(got, "# Skills") || !strings.Contains(got, "Skills available:") || !strings.Contains(got, "load_skill") {
		t.Fatalf("unexpected skills section:\n%s", got)
	}
}

func TestProjectInstructionsSection(t *testing.T) {
	prompt := New(Config{
		ProjectInstructions: []ProjectInstruction{
			{Path: "/repo/AGENTS.md", Content: "Root rules."},
			{Path: "/repo/pkg/CLAUDE.md", Content: "Package rules.", Truncated: true},
		},
	}).Build()
	for _, want := range []string{"# Project Instructions", "/repo/AGENTS.md", "Root rules.", "/repo/pkg/CLAUDE.md (truncated)", "Package rules."} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestSystemSection(t *testing.T) {
	prompt := New(Config{
		Identity: "Test agent",
		Tools:    ToolSet{Shell: true},
	}).Build()
	for _, want := range []string{
		"# System",
		"<system-reminder>",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestDoingTasksSection(t *testing.T) {
	prompt := New(Config{
		Identity: "Test agent",
		Tools:    ToolSet{Shell: true},
	}).Build()
	for _, want := range []string{
		"# Doing Tasks",
		"Do not propose changes to code you haven't read",
		"Do not create files unless they are absolutely necessary",
		"at most two materially different fixes",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestSafetySection(t *testing.T) {
	prompt := New(Config{
		Identity: "Test agent",
		Tools:    ToolSet{Shell: true},
	}).Build()
	for _, want := range []string{
		"# Safety",
		"Do not generate or guess URLs",
		"Tool calls may require user approval",
		"prompt injection",
		"security vulnerabilities",
		"OWASP Top 10",
		"reversibility and blast radius",
		"destructive",
		"force-pushing",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestCommunicationSection(t *testing.T) {
	prompt := New(Config{
		Identity: "Test agent",
		Tools:    ToolSet{Shell: true},
	}).Build()
	for _, want := range []string{
		"# Communication",
		"Only use emojis",
		"Be concise and direct",
		"file_path:line_number",
		"Do not use a colon before tool calls",
		"Decisions that need the user's input",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestLoadGitSnapshot(t *testing.T) {
	runner := fakeGitRunner{
		outputs: map[string]string{
			"git rev-parse --is-inside-work-tree":               "true",
			"git branch --show-current":                         "feature",
			"git symbolic-ref --short refs/remotes/origin/HEAD": "origin/main",
			"git config user.name":                              "Dean",
			"git --no-optional-locks status --short":            strings.Repeat("x", 20),
			"git --no-optional-locks log --oneline -n 5":        "abc test",
		},
	}
	got, err := LoadGitSnapshot(context.Background(), "/repo", runner, 10)
	if err != nil {
		t.Fatalf("LoadGitSnapshot: %v", err)
	}
	if got.CurrentBranch != "feature" || got.DefaultBranch != "main" || got.UserName != "Dean" {
		t.Fatalf("snapshot metadata: %#v", got)
	}
	if len([]rune(got.Status)) != 10 || !got.Truncated {
		t.Fatalf("status truncation: %#v", got)
	}
}

func TestLoadGitSnapshotNotRepo(t *testing.T) {
	_, err := LoadGitSnapshot(context.Background(), "/repo", fakeGitRunner{err: errors.New("not repo")}, 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultEnvironment(t *testing.T) {
	got := DefaultEnvironment("/repo", time.Date(2026, 5, 9, 1, 2, 3, 0, time.UTC), true, true)
	if got.Date != "2026-05-09" || got.Workdir != "/repo" || !got.IsGit || !got.IsGitKnown {
		t.Fatalf("DefaultEnvironment = %#v", got)
	}
}

type fakeGitRunner struct {
	outputs map[string]string
	err     error
}

func (f fakeGitRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	key := name + " " + strings.Join(args, " ")
	return f.outputs[key], nil
}
