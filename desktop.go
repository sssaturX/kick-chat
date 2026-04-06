//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

func minimizeConsoleIfPossible() {}

func openDashboardBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}
