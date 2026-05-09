package prompt

import (
	"strings"
	"testing"
)

func TestSkillsSection(t *testing.T) {
	section := SkillsSection("  - demo: test skill")
	got := section.String()
	if !strings.Contains(got, "# Skills") || !strings.Contains(got, "Skills available:") || !strings.Contains(got, "load_skill") {
		t.Fatalf("unexpected skills section:\n%s", got)
	}
}

func TestRuntimeSectionsAreGeneric(t *testing.T) {
	prompt := Join("", RuntimeSections(RuntimeConfig{SkillsSummary: "- demo: test skill"}))
	for _, want := range []string{"# Skills", "demo: test skill"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("runtime prompt missing %q:\n%s", want, prompt)
		}
	}
	for _, notWant := range []string{"# Identity", "# Doing Tasks", "# Safety", "# Environment", "# Project Instructions", "# Tool Guidance", "# Communication"} {
		if strings.Contains(prompt, notWant) {
			t.Fatalf("runtime prompt should not contain coding-agent section %q:\n%s", notWant, prompt)
		}
	}
}

func TestJoinSkipsEmptySections(t *testing.T) {
	got := Join(" base ", []Section{
		{Name: "Empty", Content: "  "},
		{Name: "Rules", Content: "Use tools."},
		{Name: "", Content: "Plain text."},
	})
	for _, want := range []string{"base", "# Rules\nUse tools.", "Plain text."} {
		if !strings.Contains(got, want) {
			t.Fatalf("joined prompt missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Empty") {
		t.Fatalf("joined prompt contains empty section:\n%s", got)
	}
}
