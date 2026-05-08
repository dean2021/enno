package filesystem

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilesystemReadHonorsMaxOutputChars(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "long.txt"), []byte(strings.Repeat("a", 100)), 0644); err != nil {
		t.Fatal(err)
	}
	fs := &Filesystem{root: dir, maxOutputChars: 30}
	out, err := fs.Read("long.txt", 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len([]rune(out)) > 30 {
		t.Fatalf("output len = %d, want <= 30", len([]rune(out)))
	}
	if !strings.Contains(out, "[truncated]") {
		t.Fatalf("expected truncation suffix, got %q", out)
	}
}

func TestFilesystemSafePathRejectsEscape(t *testing.T) {
	fs := &Filesystem{root: t.TempDir(), maxOutputChars: 100}
	if _, err := fs.safePath("../../../etc/passwd"); err == nil {
		t.Fatal("expected path escape error")
	}
}
