//go:build release

package viewerbot

import (
	"os"
	"path/filepath"
	"runtime"
)

// resolveViewerbotRunner в release ищет только бинарник viewerbot (PyInstaller) — без .py, код не отдаётся.
func resolveViewerbotRunner() (path string, isPython bool) {
	binName := "viewerbot"
	if runtime.GOOS == "windows" {
		binName = "viewerbot.exe"
	}
	tryDir := func(dir string) string {
		p := filepath.Join(dir, binName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	if p := os.Getenv("VIEWBOT_BIN"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, false
		}
	}
	if exe, err := os.Executable(); err == nil {
		if p := tryDir(filepath.Dir(exe)); p != "" {
			return p, false
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if p := tryDir(cwd); p != "" {
			return p, false
		}
	}
	return "", false
}
