package projectrules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRootFirstAndCloserLater(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "sub")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "AGENTS.md"), "root agents")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "root claude")
	writeFile(t, filepath.Join(root, "pkg", "AGENTS.md"), "pkg agents")

	got, err := Load(Config{Workdir: nested})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, got %#v", len(got), got)
	}
	wantSuffixes := []string{
		filepath.Join("AGENTS.md"),
		filepath.Join("CLAUDE.md"),
		filepath.Join("pkg", "AGENTS.md"),
	}
	for i, suffix := range wantSuffixes {
		if !strings.HasSuffix(got[i].Path, suffix) {
			t.Fatalf("entry %d path = %q, want suffix %q", i, got[i].Path, suffix)
		}
	}
}

func TestLoadSkipsDuplicateContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "AGENTS.md"), "same")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "same")

	got, err := Load(Config{Workdir: root})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, got %#v", len(got), got)
	}
}

func TestLoadTruncatesByFileAndTotalBudget(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "AGENTS.md"), "abcdef")
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "ghijkl")

	got, err := Load(Config{Workdir: root, MaxFileChars: 4, MaxTotalChars: 6})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, got %#v", len(got), got)
	}
	if got[0].Content != "abcd" || !got[0].Truncated {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1].Content != "gh" || !got[1].Truncated {
		t.Fatalf("second = %#v", got[1])
	}
}

func TestLoadMissingFiles(t *testing.T) {
	got, err := Load(Config{Workdir: t.TempDir()})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestLoadSkipsUnreadableFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	writeFile(t, path, "ok")
	if err := os.Chmod(path, 0000); err != nil {
		t.Skip(err)
	}
	defer os.Chmod(path, 0644)

	got, err := Load(Config{Workdir: root})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected unreadable file to be skipped, got %#v", got)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
