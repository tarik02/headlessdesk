//go:build windows

package servercmd

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const swHide = 0

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procGetConsoleWindow      = kernel32.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procShowWindow            = user32.NewProc("ShowWindow")
)

func hideStandaloneConsole() {
	window, _, _ := procGetConsoleWindow.Call()
	if window == 0 {
		return
	}

	var processIDs [8]uint32
	count, _, _ := procGetConsoleProcessList.Call(
		uintptr(unsafe.Pointer(&processIDs[0])),
		uintptr(len(processIDs)),
	)
	if count != 1 {
		return
	}

	procShowWindow.Call(window, swHide)
}
