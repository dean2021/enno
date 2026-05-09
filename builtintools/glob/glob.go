// Package glob provides a read-only file listing tool backed by ripgrep (rg --files).
package glob

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dean2021/enno"
	"github.com/dean2021/enno/builtintools/internal/toolutil"
)

// ToolName is the registered tool identifier (snake_case, consistent with other built-in tools).
const ToolName = "glob"

const defaultFileLimit = 100

// Config scopes search under Root (same idea as filesystem.Config). Timeout bounds each invocation.
type Config struct {
	Root           string
	Timeout        time.Duration
	MaxOutputChars int
}

type globTool struct {
	root           string
	timeout        time.Duration
	maxOutputChars int
}

type args struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	// Limit nil = default 100; non-nil 0 = unlimited; else max number of paths returned.
	Limit  *int `json:"limit"`
	Offset int  `json:"offset"`
}

const toolDescription = `Fast file pattern matching (glob) via ripgrep --files. Use for finding paths by name/wildcard; use the grep tool for searching file contents.

- pattern: glob passed to rg --glob (e.g. "**/*.go"). Absolute patterns split into base directory + relative glob (same idea as Claude Code).
- path: optional directory under the workspace root; omit or "." to search from the workspace root.
- limit: max paths returned (default 100); 0 means unlimited (can be large).
- offset: skip this many matching paths before applying limit.

Requires ripgrep ("rg") on PATH.`

// New returns a single Tool named ToolName ("glob").
func New(config Config) enno.Tool {
	root := config.Root
	if root == "" {
		root = "."
	}
	g := &globTool{
		root:           root,
		timeout:        toolutil.Timeout(config.Timeout),
		maxOutputChars: toolutil.MaxOutputChars(config.MaxOutputChars),
	}

	props := map[string]any{
		"pattern": map[string]any{"type": "string", "description": "Glob pattern for ripgrep --glob."},
		"path": map[string]any{
			"type":        "string",
			"description": "Optional subdirectory relative to workspace root; omit to use the whole workspace.",
		},
		"limit": map[string]any{
			"type":        "integer",
			"description": "Max paths returned (default 100). Use 0 for unlimited.",
		},
		"offset": map[string]any{"type": "integer", "description": "Skip this many paths before applying limit."},
	}

	return enno.NewTypedTool(ToolName, toolDescription, props, []string{"pattern"}, g.run)
}

func (g *globTool) run(ctx context.Context, a args) (string, error) {
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

	searchDirAbs, err := g.resolveSearchDir(rootAbs, a.Path)
	if err != nil {
		return "", err
	}

	patternTrim := strings.TrimSpace(a.Pattern)

	var cmdDir string
	var globArg string

	if filepath.IsAbs(patternTrim) {
		baseDir, relPat := extractGlobBaseDirectory(patternTrim)
		if baseDir != "" {
			absBase, err := filepath.Abs(baseDir)
			if err != nil {
				return "", err
			}
			absBase = filepath.Clean(absBase)
			if !underRoot(rootAbs, absBase) {
				return "", fmt.Errorf("pattern base escapes workspace root: %s", patternTrim)
			}
			cmdDir = absBase
			globArg = relPat
		} else {
			cmdDir = searchDirAbs
			globArg = patternTrim
		}
	} else {
		cmdDir = searchDirAbs
		globArg = patternTrim
	}

	if !underRoot(rootAbs, cmdDir) {
		return "", fmt.Errorf("search directory escapes workspace root")
	}

	st, err := os.Stat(cmdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", a.Path)
		}
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", strings.TrimSpace(a.Path))
	}

	rgArgs := []string{
		"--color", "never",
		"--files",
		"--glob", globArg,
		"--sort=modified",
		"--no-ignore",
		"--hidden",
		".",
	}

	runCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "rg", rgArgs...)
	cmd.Dir = cmdDir
	combined, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("glob timeout (%s)", g.timeout)
	}
	outStr := string(combined)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ProcessState != nil && ee.ProcessState.ExitCode() == 1 {
			return "No files found", nil
		}
		return "", fmt.Errorf("rg: %w\n%s", err, strings.TrimSpace(outStr))
	}

	lines := strings.Split(strings.TrimSuffix(outStr, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	absPaths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		joined := line
		if !filepath.IsAbs(line) {
			joined = filepath.Join(cmdDir, line)
		}
		joined = filepath.Clean(joined)
		rel, err := filepath.Rel(rootAbs, joined)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue
		}
		absPaths = append(absPaths, rel)
	}

	limit := defaultFileLimit
	if a.Limit != nil {
		if *a.Limit == 0 {
			limit = -1
		} else {
			limit = *a.Limit
		}
	}
	offset := a.Offset
	if offset < 0 {
		offset = 0
	}

	total := len(absPaths)
	if total == 0 {
		return "No files found", nil
	}

	truncated := limit >= 0 && total > offset+limit
	var slice []string
	if offset >= total {
		return "No files found", nil
	}
	end := total
	if limit >= 0 {
		end = offset + limit
		if end > total {
			end = total
		}
	}
	slice = absPaths[offset:end]

	out := strings.Join(slice, "\n")
	if truncated {
		out += "\n\n(Results are truncated. Consider using a more specific path or pattern.)"
	}
	return toolutil.TruncateRunes(out, g.maxOutputChars, toolutil.DefaultTruncationSuffix), nil
}

func (g *globTool) resolveSearchDir(rootAbs, path string) (string, error) {
	t := strings.TrimSpace(path)
	if t == "" || t == "." {
		return rootAbs, nil
	}
	abs, err := g.safePath(t)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if !underRoot(rootAbs, abs) {
		return "", fmt.Errorf("path escapes root: %s", path)
	}
	return abs, nil
}

func (g *globTool) safePath(path string) (string, error) {
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

func underRoot(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// extractGlobBaseDirectory splits pattern into a static directory prefix and the remaining glob.
// Ported from Claude Code utils/glob.ts (simplified: no Windows drive-root special case).
func extractGlobBaseDirectory(pattern string) (baseDir, relativePattern string) {
	idx := -1
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[', '{':
			idx = i
			goto found
		}
	}
found:
	if idx < 0 {
		dir, file := filepath.Split(pattern)
		dir = strings.TrimSuffix(dir, string(filepath.Separator))
		if dir == "" {
			return ".", file
		}
		return dir, file
	}

	staticPrefix := pattern[:idx]
	lastSep := strings.LastIndex(staticPrefix, "/")
	lastSepWin := strings.LastIndex(staticPrefix, `\`)
	if lastSepWin > lastSep {
		lastSep = lastSepWin
	}
	if lastSep < 0 {
		return "", pattern
	}

	baseDir = staticPrefix[:lastSep]
	relativePattern = pattern[lastSep+1:]
	if baseDir == "" && lastSep == 0 {
		baseDir = string(filepath.Separator)
	}
	return baseDir, relativePattern
}
