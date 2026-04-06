//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func minimizeConsoleIfPossible() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	const swMinimize = 6
	_, _, _ = showWindow.Call(hwnd, uintptr(swMinimize))
}

func openDashboardBrowser(url string) {
	_ = exec.Command("cmd", "/c", "start", "", url).Start()
}
