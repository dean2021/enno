// Package grep provides a read-only content search tool backed by ripgrep (rg).
package grep

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/internal/builtintools/internal/toolutil"
)

// ToolName is the registered tool identifier (snake_case, consistent with other built-in tools).
const ToolName = "grep"

const defaultHeadLimit = 250

// Config scopes search under Root (same idea as filesystem.Config). Timeout bounds each invocation.
type Config struct {
	Root           string
	Timeout        time.Duration
	MaxOutputChars int
}

type grepTool struct {
	root           string
	timeout        time.Duration
	maxOutputChars int
}

type args struct {
	Pattern string `json:"pattern"`

	Path string `json:"path"`

	Glob string `json:"glob"`

	// OutputMode: "files_with_matches" (default), "content", or "count".
	OutputMode string `json:"output_mode"`

	CaseInsensitive bool `json:"case_insensitive"`

	Context int `json:"context"`
	Before  int `json:"before"`
	After   int `json:"after"`

	// LineNumbers defaults to true when OutputMode is content and Field is omitted.
	LineNumbers *bool `json:"line_numbers"`

	Type string `json:"type"`

	Multiline bool `json:"multiline"`

	// HeadLimit nil = default 250 lines; non-nil: 0 = unlimited, else cap.
	HeadLimit *int `json:"head_limit"`
	Offset    int  `json:"offset"`
}

// toolDescription is model-facing text (short; mirrors Claude Code guidance).
const toolDescription = `Search file contents using ripgrep (regex). Use this tool instead of invoking grep/rg via the shell.

- pattern: regex (ripgrep syntax, not POSIX grep).
- path: optional file or directory under the workspace root; default searches the whole workspace.
- output_mode: "files_with_matches" (default), "content" (matching lines), or "count" (match counts per file).
- glob / type: filter files like rg --glob and --type.
- context / before / after: only used with output_mode "content" (-C / -B / -A).
- head_limit: max output lines (default 250); 0 means unlimited.
Requires ripgrep ("rg") on PATH.`

// New returns a single Tool named ToolName ("grep").
func New(config Config) enno.Tool {
	root := config.Root
	if root == "" {
		root = "."
	}
	g := &grepTool{
		root:           root,
		timeout:        toolutil.Timeout(config.Timeout),
		maxOutputChars: toolutil.MaxOutputChars(config.MaxOutputChars),
	}

	props := map[string]any{
		"pattern": map[string]any{"type": "string", "description": "Regex pattern (ripgrep)."},
		"path":    map[string]any{"type": "string", "description": "Optional path relative to workspace root; empty searches entire workspace."},
		"glob":    map[string]any{"type": "string", "description": "Optional glob filter, e.g. *.go."},
		"output_mode": map[string]any{
			"type":        "string",
			"enum":        []string{"files_with_matches", "content", "count"},
			"description": `Default files_with_matches. "content" shows lines; "count" shows per-file counts.`,
		},
		"case_insensitive": map[string]any{"type": "boolean"},
		"context":          map[string]any{"type": "integer", "description": "Lines before+after each match (content mode)."},
		"before":           map[string]any{"type": "integer", "description": "Lines before match (content mode)."},
		"after":            map[string]any{"type": "integer", "description": "Lines after match (content mode)."},
		"line_numbers":     map[string]any{"type": "boolean", "description": "Show line numbers in content mode; default true."},
		"type":             map[string]any{"type": "string", "description": "Ripgrep --type, e.g. go, py."},
		"multiline":        map[string]any{"type": "boolean", "description": "Multiline dotall matching."},
		"head_limit": map[string]any{
			"type":        "integer",
			"description": "Max lines of output; default 250. Use 0 for unlimited.",
		},
		"offset": map[string]any{"type": "integer", "description": "Skip this many lines before applying head_limit."},
	}

	return enno.NewTypedTool(ToolName, toolDescription, props, []string{"pattern"}, g.run)
}

func (g *grepTool) run(ctx context.Context, a args) (string, error) {
	if strings.TrimSpace(a.Pattern) == "" {
		return "", fmt.Errorf("missing pattern")
	}
	if _, err := exec.LookPath("rg"); err != nil {
		return "", fmt.Errorf(`ripgrep ("rg") not found in PATH; install from https://github.com/BurntSushi/ripgrep`)
	}

	rootAbs, err := filepath.Abs(g.root)
	if err != nil {
		return "", err
	}

	relSearch, err := g.searchPathRelative(rootAbs, a.Path)
	if err != nil {
		return "", err
	}

	mode := strings.TrimSpace(strings.ToLower(a.OutputMode))
	if mode == "" {
		mode = "files_with_matches"
	}

	var rgArgs []string
	rgArgs = append(rgArgs, "--color", "never")

	switch mode {
	case "files_with_matches":
		rgArgs = append(rgArgs, "-l")
	case "content":
		showNums := a.LineNumbers == nil || *a.LineNumbers
		if showNums {
			rgArgs = append(rgArgs, "-n")
		} else {
			rgArgs = append(rgArgs, "--no-line-number")
		}
		switch {
		case a.Context > 0:
			rgArgs = append(rgArgs, "-C", strconv.Itoa(a.Context))
		default:
			if a.Before > 0 {
				rgArgs = append(rgArgs, "-B", strconv.Itoa(a.Before))
			}
			if a.After > 0 {
				rgArgs = append(rgArgs, "-A", strconv.Itoa(a.After))
			}
		}
	case "count":
		rgArgs = append(rgArgs, "-c")
	default:
		return "", fmt.Errorf("invalid output_mode %q (want files_with_matches, content, or count)", a.OutputMode)
	}

	if a.CaseInsensitive {
		rgArgs = append(rgArgs, "-i")
	}
	if a.Multiline {
		rgArgs = append(rgArgs, "-U", "--multiline-dotall")
	}
	if strings.TrimSpace(a.Glob) != "" {
		rgArgs = append(rgArgs, "--glob", strings.TrimSpace(a.Glob))
	}
	if strings.TrimSpace(a.Type) != "" {
		rgArgs = append(rgArgs, "--type", strings.TrimSpace(a.Type))
	}

	rgArgs = append(rgArgs, "--", a.Pattern, relSearch)

	runCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "rg", rgArgs...)
	cmd.Dir = rootAbs
	combined, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("grep timeout (%s)", g.timeout)
	}
	outStr := string(combined)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ProcessState != nil && ee.ProcessState.ExitCode() == 1 {
			return "(no matches)", nil
		}
		return "", fmt.Errorf("rg: %w\n%s", err, strings.TrimSpace(outStr))
	}

	lines := strings.Split(strings.TrimSuffix(outStr, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	limit := defaultHeadLimit
	if a.HeadLimit != nil {
		if *a.HeadLimit == 0 {
			limit = -1
		} else {
			limit = *a.HeadLimit
		}
	}
	offset := a.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(lines) {
		return "(no matches in offset window)", nil
	}
	sliced := lines[offset:]
	truncated := false
	if limit >= 0 && len(sliced) > limit {
		sliced = sliced[:limit]
		truncated = true
	}
	text := strings.Join(sliced, "\n")
	if truncated {
		text += fmt.Sprintf("\n\n[Output truncated: showing %d lines from offset %d; adjust head_limit/offset or narrow the search.]", limit, offset)
	}
	if strings.TrimSpace(text) == "" {
		return "(no matches)", nil
	}
	return toolutil.TruncateRunes(text, g.maxOutputChars, toolutil.DefaultTruncationSuffix), nil
}

func (g *grepTool) searchPathRelative(rootAbs, path string) (string, error) {
	t := strings.TrimSpace(path)
	if t == "" || t == "." {
		return ".", nil
	}
	abs, err := g.safePath(t)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root: %s", path)
	}
	if rel == "." {
		return ".", nil
	}
	return rel, nil
}

func (g *grepTool) safePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("missing path")
	}
	root, err := filepath.Abs(g.root)
	if err != nil {
		return "", err
	}
	var target string
	if filepath.IsAbs(path) {
		target = filepath.Clean(path)
	} else {
		target = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes root: %s", path)
	}
	return target, nil
}
