package glob

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

func TestGlobToolSafePathRejectsEscape(t *testing.T) {
	g := &globTool{root: t.TempDir(), timeout: time.Second}
	_, err := g.safePath("../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path escape")
	}
}

func TestExtractGlobBaseDirectory(t *testing.T) {
	cases := []struct {
		in, wantBase, wantRel string
	}{
		{"**/*.go", "", "**/*.go"},
		{"/a/b/*.txt", "/a/b", "*.txt"},
		{filepath.Join("foo", "bar.go"), "foo", "bar.go"},
	}
	for _, tc := range cases {
		b, r := extractGlobBaseDirectory(tc.in)
		if b != tc.wantBase || r != tc.wantRel {
			t.Errorf("%q: got base=%q rel=%q want base=%q rel=%q", tc.in, b, r, tc.wantBase, tc.wantRel)
		}
	}
}

func TestGlobRipgrepIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skip rg integration in short mode")
	}
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := New(Config{Root: dir, Timeout: 5 * time.Second})
	if tool.Name != ToolName {
		t.Fatalf("name %q", tool.Name)
	}
	raw := json.RawMessage(`{"pattern":"*.go","path":"."}`)
	ctx := context.Background()
	out, err := tool.Handler(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello.go") {
		t.Fatalf("unexpected output: %q", out)
	}
}
