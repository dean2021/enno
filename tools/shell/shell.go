package shell

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dean2021/enno"
)

type Config struct {
	Workdir  string
	Timeout  time.Duration
	DenyList []string
}

type args struct {
	Command string `json:"command"`
}

type Shell struct {
	workdir  string
	timeout  time.Duration
	denyList []string
}

func New(config Config) enno.Tool {
	sh := &Shell{
		workdir: config.Workdir,
		timeout: config.Timeout,
		denyList: []string{
			"rm -rf /",
			"sudo",
			"shutdown",
			"reboot",
			"> /dev/",
		},
	}
	if sh.timeout == 0 {
		sh.timeout = 120 * time.Second
	}
	if len(config.DenyList) > 0 {
		sh.denyList = append([]string(nil), config.DenyList...)
	}

	return enno.NewTypedTool("bash", "Run a shell command.", map[string]any{
		"command": map[string]any{"type": "string"},
	}, []string{"command"}, func(ctx context.Context, input args) (string, error) {
		if strings.TrimSpace(input.Command) == "" {
			return "", fmt.Errorf("missing command")
		}
		return sh.Run(ctx, input.Command)
	})
}

func (s *Shell) Run(ctx context.Context, command string) (string, error) {
	for _, pattern := range s.denyList {
		if strings.Contains(command, pattern) {
			return "", fmt.Errorf("dangerous command blocked")
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

	return truncate(output, 50000), nil
}

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
