package projectrules

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultMaxFileChars  = 20000
	defaultMaxTotalChars = 60000
)

var defaultFileNames = []string{"AGENTS.md", "CLAUDE.md"}

type Config struct {
	Workdir       string
	FileNames     []string
	MaxFileChars  int
	MaxTotalChars int
}

type Instruction struct {
	Path      string
	Content   string
	Truncated bool
}

func Load(config Config) ([]Instruction, error) {
	workdir := strings.TrimSpace(config.Workdir)
	if workdir == "" {
		workdir = "."
	}
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return nil, err
	}
	dirs, err := directoriesRootFirst(abs)
	if err != nil {
		return nil, err
	}

	fileNames := append([]string(nil), config.FileNames...)
	useDefaultPriority := len(fileNames) == 0
	if useDefaultPriority {
		fileNames = append([]string(nil), defaultFileNames...)
	}
	maxFileChars := config.MaxFileChars
	if maxFileChars <= 0 {
		maxFileChars = defaultMaxFileChars
	}
	maxTotalChars := config.MaxTotalChars
	if maxTotalChars <= 0 {
		maxTotalChars = defaultMaxTotalChars
	}

	seenPaths := make(map[string]bool)
	seenContent := make(map[string]bool)
	var total int
	var instructions []Instruction
	for _, dir := range dirs {
		names := fileNames
		if useDefaultPriority {
			names = firstExistingRuleFile(dir, fileNames)
		}
		for _, name := range names {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			path := filepath.Join(dir, name)
			info, err := os.Stat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				continue
			}
			if info.IsDir() {
				continue
			}
			key := canonicalPath(path)
			if seenPaths[key] {
				continue
			}
			seenPaths[key] = true

			bytes, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			content := strings.TrimSpace(string(bytes))
			if content == "" {
				continue
			}
			hash := contentHash(content)
			if seenContent[hash] {
				continue
			}
			seenContent[hash] = true

			remaining := maxTotalChars - total
			if remaining <= 0 {
				return instructions, nil
			}
			limit := maxFileChars
			if remaining < limit {
				limit = remaining
			}
			content, truncated := truncateRunes(content, limit)
			total += len([]rune(content))
			instructions = append(instructions, Instruction{
				Path:      path,
				Content:   content,
				Truncated: truncated,
			})
		}
	}
	return instructions, nil
}

func firstExistingRuleFile(dir string, fileNames []string) []string {
	for _, name := range fileNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.IsDir() {
			continue
		}
		return []string{name}
	}
	return nil
}

func directoriesRootFirst(abs string) ([]string, error) {
	var dirs []string
	current := filepath.Clean(abs)
	for {
		info, err := os.Stat(current)
		if err == nil && !info.IsDir() {
			current = filepath.Dir(current)
		}
		dirs = append(dirs, current)
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	for left, right := 0, len(dirs)-1; left < right; left, right = left+1, right-1 {
		dirs[left], dirs[right] = dirs[right], dirs[left]
	}
	return dirs, nil
}

func canonicalPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		return abs
	}
	return path
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func truncateRunes(content string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return "", true
	}
	runes := []rune(content)
	if len(runes) <= maxChars {
		return content, false
	}
	return string(runes[:maxChars]), true
}
