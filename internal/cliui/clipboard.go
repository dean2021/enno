package cliui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// writeSystemClipboard copies text to the OS clipboard when a suitable helper exists.
func writeSystemClipboard(text string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	case "linux":
		if path, err := exec.LookPath("xclip"); err == nil {
			cmd := exec.Command(path, "-selection", "clipboard")
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		if path, err := exec.LookPath("wl-copy"); err == nil {
			cmd := exec.Command(path)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
		return fmt.Errorf("need xclip or wl-copy on PATH for clipboard on Linux")
	case "windows":
		cmd := exec.Command("clip")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	default:
		return fmt.Errorf("clipboard not supported on %s", runtime.GOOS)
	}
}
