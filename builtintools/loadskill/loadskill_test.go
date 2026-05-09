package loadskill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillFileWithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "pdf", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: my-pdf\ndescription: Handle PDFs\n---\n\nFirst line of body.\n"
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	name, desc, body, err := parseSkillFile(p, "pdf")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if name != "my-pdf" {
		t.Fatalf("name: %q", name)
	}
	if desc != "Handle PDFs" {
		t.Fatalf("desc: %q", desc)
	}
	if !strings.Contains(body, "First line of body") {
		t.Fatalf("body: %q", body)
	}
}

func TestParseSkillFileNoFrontmatter(t *testing.T) {
	p := filepath.Join(t.TempDir(), "raw", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("only body\n"), 0644); err != nil {
		t.Fatal(err)
	}
	name, _, body, err := parseSkillFile(p, "raw")
	if err != nil {
		t.Fatal(err)
	}
	if name != "raw" {
		t.Fatalf("name: %q", name)
	}
	if body != "only body" {
		t.Fatalf("body: %q", body)
	}
}

func TestLoadDirAndDuplicateNameLastWins(t *testing.T) {
	root := t.TempDir()
	// a/pdf/SKILL.md
	writeSkill(t, filepath.Join(root, "a", "pdf"), "name: x\ndescription: from-a\n", "A")
	// b/pdf/SKILL.md — same meta name "x" but different path; last path wins
	writeSkill(t, filepath.Join(root, "b", "pdf"), "name: x\ndescription: from-b\n", "B")

	r, err := LoadDir(root)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if r.Count() != 1 {
		t.Fatalf("expected 1 skill, got %d", r.Count())
	}
	e := r.byName["x"]
	if !strings.Contains(e.Body, "B") {
		t.Fatalf("expected last path to win, got body %q", e.Body)
	}
	if e.Description != "from-b" {
		t.Fatalf("description: %q", e.Description)
	}
}

func TestRegistryDescriptionsAndGetContent(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "one"), "name: alpha\ndescription: first\n", "body1")
	writeSkill(t, filepath.Join(root, "two"), "name: beta\ndescription: second\n", "body2")

	r, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	d := r.DescriptionsText()
	if !strings.Contains(d, "  - alpha: first") || !strings.Contains(d, "  - beta: second") {
		t.Fatalf("descriptions: %q", d)
	}

	got := r.GetContent("alpha")
	if !strings.HasPrefix(got, "<skill name=\"alpha\">") || !strings.Contains(got, "body1") {
		t.Fatalf("get alpha: %q", got)
	}
	if r.GetContent("nope") != "Error: Unknown skill 'nope'." {
		t.Fatalf("unknown: %q", r.GetContent("nope"))
	}
}

func TestLoadDirsMergesAndOverlays(t *testing.T) {
	base := t.TempDir()
	writeSkill(t, filepath.Join(base, "a", "s1"), "name: one\ndescription: A\n", "A")
	extra := t.TempDir()
	writeSkill(t, filepath.Join(extra, "b", "s1"), "name: one\ndescription: B\n", "B")

	r, err := LoadDirs([]string{base, extra})
	if err != nil {
		t.Fatalf("LoadDirs: %v", err)
	}
	if r.byName["one"].Description != "B" {
		t.Fatalf("expected second dir to override, got %#v", r.byName["one"])
	}
}

func TestLoadDirsSkipsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "x"), "name: y\ndescription: d\n", "b")

	r, err := LoadDirs([]string{missing, root})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count() != 1 {
		t.Fatalf("expected 1 skill, got %d", r.Count())
	}
}

func TestNewTool(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "s"), "name: t\ndescription: d\n", "body")
	r, err := LoadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	tool, err := NewTool(r)
	if err != nil {
		t.Fatal(err)
	}
	if tool.Name != "load_skill" {
		t.Fatalf("name: %q", tool.Name)
	}
	out, err := tool.Handler(context.Background(), []byte(`{"name":"t"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "body") {
		t.Fatalf("out: %q", out)
	}
}

func writeSkill(t *testing.T, dir, front, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	fm := "---\n" + strings.TrimSpace(front) + "\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(fm), 0644); err != nil {
		t.Fatal(err)
	}
}
