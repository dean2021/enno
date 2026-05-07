package loadskill

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const skillFileName = "SKILL.md"

// Entry is one loaded skill.
type Entry struct {
	Name        string
	Description string
	Body        string
	SourcePath  string
}

// Registry holds skills indexed by name. If the same name appears in multiple files,
// the lexicographically last path wins (deterministic).
type Registry struct {
	byName map[string]Entry
}

// LoadDir walks root recursively and loads every SKILL.md. root must exist and be a directory.
// Returns an empty registry (not nil) if no files are found. Duplicate names: last path wins.
func LoadDir(root string) (*Registry, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills path is not a directory: %s", root)
	}

	var paths []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == skillFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)
	r := &Registry{byName: make(map[string]Entry)}
	for _, p := range paths {
		defaultName := filepath.Base(filepath.Dir(p))
		name, desc, body, perr := parseSkillFile(p, defaultName)
		if perr != nil {
			return nil, perr
		}
		if name == "" {
			continue
		}
		r.byName[name] = Entry{
			Name:        name,
			Description: desc,
			Body:        body,
			SourcePath:  p,
		}
	}
	return r, nil
}

// LoadDirs merges skills from multiple directories in order. Paths that do not exist are skipped
// without error. Later directories override earlier entries when the same skill name appears.
func LoadDirs(roots []string) (*Registry, error) {
	merged := &Registry{byName: make(map[string]Entry)}
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("skills path %s: %w", root, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("skills path is not a directory: %s", root)
		}
		r, err := LoadDir(root)
		if err != nil {
			return nil, fmt.Errorf("load skills from %s: %w", root, err)
		}
		for name, e := range r.byName {
			merged.byName[name] = e
		}
	}
	return merged, nil
}

// Count returns the number of registered skills.
func (r *Registry) Count() int {
	if r == nil {
		return 0
	}
	return len(r.byName)
}

// DescriptionsText returns lines "  - name: description" sorted by name, or "" if empty.
func (r *Registry) DescriptionsText() string {
	if r == nil || len(r.byName) == 0 {
		return ""
	}
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for i, n := range names {
		if i > 0 {
			b.WriteString("\n")
		}
		e := r.byName[n]
		b.WriteString("  - ")
		b.WriteString(e.Name)
		b.WriteString(": ")
		b.WriteString(e.Description)
	}
	return b.String()
}

// GetContent returns the full skill body wrapped in XML, or an error line for unknown names.
func (r *Registry) GetContent(name string) string {
	if r == nil {
		return fmt.Sprintf("Error: Unknown skill '%s'.", name)
	}
	e, ok := r.byName[strings.TrimSpace(name)]
	if !ok {
		return fmt.Sprintf("Error: Unknown skill '%s'.", name)
	}
	return fmt.Sprintf("<skill name=\"%s\">\n%s\n</skill>", xmlEscapeAttr(e.Name), e.Body)
}

func xmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
