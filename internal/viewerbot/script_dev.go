//go:build !release

package viewerbot

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveViewerbotRunner returns path to viewerbot runner: сначала бинарник (viewerbot/viewerbot.exe), потом kick.py.
// Бинарник — без Python у юзера; .py — для разработки.
func resolveViewerbotRunner() (path string, isPython bool) {
	tryDir := func(dir string) (string, bool) {
		// 1) Бинарник (скомпилированный PyInstaller — код не виден)
		binName := "viewerbot"
		if runtime.GOOS == "windows" {
			binName = "viewerbot.exe"
		}
		if p := filepath.Join(dir, binName); pathExists(p) {
			return p, false
		}
		// 2) .py для разработки
		for _, n := range []string{"test_view/kick-viewbot/kick.py", "kick-viewbot/kick.py", "kick.py"} {
			p := filepath.Join(dir, n)
			if pathExists(p) {
				return p, true
			}
		}
		return "", false
	}
	if p := os.Getenv("VIEWBOT_BIN"); p != "" && pathExists(p) {
		return p, false
	}
	if p := os.Getenv("VIEWBOT_SCRIPT"); p != "" && pathExists(p) {
		return p, strings.HasSuffix(strings.ToLower(p), ".py")
	}
	if exe, err := os.Executable(); err == nil {
		if path, py := tryDir(filepath.Dir(exe)); path != "" {
			return path, py
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		if path, py := tryDir(cwd); path != "" {
			return path, py
		}
	}
	return "", false
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
