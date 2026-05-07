package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dean2021/enno"
)

type Config struct {
	Root string
}

type readArgs struct {
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

type writeArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type editArgs struct {
	Path    string `json:"path"`
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

type Filesystem struct {
	root string
}

func New(config Config) []enno.Tool {
	fs := &Filesystem{root: config.Root}
	if fs.root == "" {
		fs.root = "."
	}
	return []enno.Tool{
		enno.NewTypedTool("read_file", "Read file contents.", map[string]any{
			"path":  map[string]any{"type": "string"},
			"limit": map[string]any{"type": "integer"},
		}, []string{"path"}, func(ctx context.Context, args readArgs) (string, error) {
			return fs.Read(args.Path, args.Limit)
		}),
		enno.NewTypedTool("write_file", "Write content to file.", map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		}, []string{"path", "content"}, func(ctx context.Context, args writeArgs) (string, error) {
			return fs.Write(args.Path, args.Content)
		}),
		enno.NewTypedTool("edit_file", "Replace exact text in file.", map[string]any{
			"path":     map[string]any{"type": "string"},
			"old_text": map[string]any{"type": "string"},
			"new_text": map[string]any{"type": "string"},
		}, []string{"path", "old_text", "new_text"}, func(ctx context.Context, args editArgs) (string, error) {
			return fs.Edit(args.Path, args.OldText, args.NewText)
		}),
	}
}

func (fs *Filesystem) Read(path string, limit int) (string, error) {
	fp, err := fs.safePath(path)
	if err != nil {
		return "", err
	}

	bytes, err := os.ReadFile(fp)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(bytes), "\n")
	if limit > 0 && limit < len(lines) {
		remaining := len(lines) - limit
		lines = append(lines[:limit], fmt.Sprintf("... (%d more lines)", remaining))
	}
	return truncate(strings.Join(lines, "\n"), 50000), nil
}

func (fs *Filesystem) Write(path, content string) (string, error) {
	fp, err := fs.safePath(path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wrote %d bytes to %s", len(content), path), nil
}

func (fs *Filesystem) Edit(path, oldText, newText string) (string, error) {
	fp, err := fs.safePath(path)
	if err != nil {
		return "", err
	}

	bytes, err := os.ReadFile(fp)
	if err != nil {
		return "", err
	}
	content := string(bytes)
	if !strings.Contains(content, oldText) {
		return "", fmt.Errorf("text not found in %s", path)
	}

	content = strings.Replace(content, oldText, newText, 1)
	if err := os.WriteFile(fp, []byte(content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Edited %s", path), nil
}

func (fs *Filesystem) safePath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("missing path")
	}

	root, err := filepath.Abs(fs.root)
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

func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
