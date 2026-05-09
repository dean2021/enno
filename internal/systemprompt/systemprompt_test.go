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
		Workdir:   "/repo",
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
		"# Environment",
		"- Current date: 2026-05-09",
		"# Tool Guidance",
		"Use read_file, write_file, and edit_file for file operations",
		"Use task_create, task_update, task_list, and task_get",
		"~/.enno/tasks/session-1/",
		"Use fetch_url",
		"Context compaction is enabled",
		"# Task Behavior",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuilderOmitsDisabledToolGuidance(t *testing.T) {
	prompt := New(Config{
		Workdir: ".",
		Tools:   ToolSet{Grep: true},
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

func TestSkillsSection(t *testing.T) {
	section := SkillsSection("  - demo: test skill")
	got := section.String()
	if !strings.Contains(got, "# Skills") || !strings.Contains(got, "Skills available:") || !strings.Contains(got, "load_skill") {
		t.Fatalf("unexpected skills section:\n%s", got)
	}
}

func TestProjectInstructionsSection(t *testing.T) {
	prompt := New(Config{
		Workdir: ".",
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
