package shell

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/builtintools/internal/toolutil"
)

type SafetyPolicy string

const (
	SafetyPolicyDenyList SafetyPolicy = "denylist"
	SafetyPolicyAllowAll SafetyPolicy = "allow_all"
)

type Config struct {
	Workdir        string
	Timeout        time.Duration
	DenyList       []string
	MaxOutputChars int
	SafetyPolicy   SafetyPolicy
}

type args struct {
	Command string `json:"command"`
}

type Shell struct {
	workdir        string
	timeout        time.Duration
	denyList       []string
	maxOutputChars int
	safetyPolicy   SafetyPolicy
}

func New(config Config) enno.Tool {
	sh := &Shell{
		workdir: config.Workdir,
		timeout: toolutil.Timeout(config.Timeout),
		denyList: []string{
			"rm -rf /",
			"sudo",
			"shutdown",
			"reboot",
			"> /dev/",
		},
		maxOutputChars: toolutil.MaxOutputChars(config.MaxOutputChars),
		safetyPolicy:   config.SafetyPolicy,
	}
	if sh.safetyPolicy == "" {
		sh.safetyPolicy = SafetyPolicyDenyList
	}
	if len(config.DenyList) > 0 {
		sh.denyList = append([]string(nil), config.DenyList...)
	}

	return enno.NewTypedTool("bash", "Run a shell command in the configured working directory. Use only when terminal execution is needed and no dedicated tool covers the task; prefer file, grep, glob, fetch_url, and task tools when they apply.", map[string]any{
		"command": map[string]any{"type": "string"},
	}, []string{"command"}, func(ctx context.Context, input args) (string, error) {
		if strings.TrimSpace(input.Command) == "" {
			return "", fmt.Errorf("missing command")
		}
		return sh.Run(ctx, input.Command)
	})
}

func (s *Shell) Run(ctx context.Context, command string) (string, error) {
	if s.safetyPolicy != SafetyPolicyAllowAll {
		for _, pattern := range s.denyList {
			if strings.Contains(command, pattern) {
				return "", fmt.Errorf("dangerous command blocked")
			}
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", command)
	if s.workdir != "" {
		cmd.Dir = s.workdir
	}

	outputBytes, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(outputBytes))
	if runCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timeout (%s)", s.timeout)
	}
	if err != nil {
		if output == "" {
			return "", err
		}
		output = output + "\n" + err.Error()
	}
	if output == "" {
		return "(no output)", nil
	}

	return toolutil.TruncateRunes(output, s.maxOutputChars, toolutil.DefaultTruncationSuffix), nil
}
