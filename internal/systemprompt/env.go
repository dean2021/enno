package systemprompt

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EnvironmentFromWorkdir(workdir string, now time.Time) Environment {
	isGit, known := IsGitRepository(workdir)
	return DefaultEnvironment(workdir, now, isGit, known)
}

func IsGitRepository(workdir string) (bool, bool) {
	dir := strings.TrimSpace(workdir)
	if dir == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
			return true, true
		} else if !os.IsNotExist(err) {
			return false, false
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return false, true
		}
		abs = parent
	}
}

func shellName() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = strings.TrimSpace(os.Getenv("ComSpec"))
	}
	if shell == "" {
		return "unknown"
	}
	base := filepath.Base(shell)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return shell
	}
	return base
}
