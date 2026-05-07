package grep

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGrepToolSafePathRejectsEscape(t *testing.T) {
	g := &grepTool{root: t.TempDir(), timeout: time.Second}
	_, err := g.safePath("../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
}

func TestGrepToolSearchPathDefault(t *testing.T) {
	root := t.TempDir()
	g := &grepTool{root: root, timeout: time.Second}
	rel, err := g.searchPathRelative(root, "")
	if err != nil || rel != "." {
		t.Fatalf("got %q %v", rel, err)
	}
	rel, err = g.searchPathRelative(root, ".")
	if err != nil || rel != "." {
		t.Fatalf("got %q %v", rel, err)
	}
}

func TestGrepToolSearchPathSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	g := &grepTool{root: root, timeout: time.Second}
	rel, err := g.searchPathRelative(root, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("a", "b")
	if rel != want {
		t.Fatalf("rel %q want %q", rel, want)
	}
}

func TestGrepRipgrepIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip rg integration in short mode")
	}
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\nfunc Foo() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := New(Config{Root: dir, Timeout: 5 * time.Second})
	if tool.Name != ToolName {
		t.Fatalf("name %q", tool.Name)
	}
	raw := json.RawMessage(`{"pattern":"func Foo","path":"."}`)
	ctx := context.Background()
	out, err := tool.Handler(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello.go") && out != "(no matches)" {
		t.Fatalf("unexpected output: %q", out)
	}
}
