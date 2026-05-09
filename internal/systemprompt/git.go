package systemprompt

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

const DefaultGitStatusLimit = 2000

type GitCommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (string, error)
}

type ExecGitRunner struct{}

func (ExecGitRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func LoadGitSnapshot(ctx context.Context, workdir string, runner GitCommandRunner, statusLimit int) (*GitSnapshot, error) {
	if runner == nil {
		runner = ExecGitRunner{}
	}
	if statusLimit <= 0 {
		statusLimit = DefaultGitStatusLimit
	}
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if _, err := runner.Run(runCtx, workdir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, err
	}

	snapshot := &GitSnapshot{}
	snapshot.CurrentBranch, _ = runner.Run(runCtx, workdir, "git", "branch", "--show-current")
	snapshot.DefaultBranch, _ = runner.Run(runCtx, workdir, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	snapshot.DefaultBranch = strings.TrimPrefix(snapshot.DefaultBranch, "origin/")
	snapshot.UserName, _ = runner.Run(runCtx, workdir, "git", "config", "user.name")
	status, _ := runner.Run(runCtx, workdir, "git", "--no-optional-locks", "status", "--short")
	snapshot.Status, snapshot.Truncated = truncateString(status, statusLimit)
	snapshot.RecentCommits, _ = runner.Run(runCtx, workdir, "git", "--no-optional-locks", "log", "--oneline", "-n", "5")
	return snapshot, nil
}

func truncateString(value string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return value, false
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value, false
	}
	return string(runes[:maxChars]), true
}
