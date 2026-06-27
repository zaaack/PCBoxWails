//go:build windows

package main

import (
	"log"
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procFindWindow   = user32.NewProc("FindWindowW")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procShowWindow    = user32.NewProc("ShowWindow")
	procIsIconic      = user32.NewProc("IsIconic")
)

const swRestore = 9

func bringWindowToFront() {
	title, _ := syscall.UTF16PtrFromString("PCBox")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		log.Println("[Window] Could not find window to bring to front")
		return
	}

	isIconic, _, _ := procIsIconic.Call(hwnd)
	if isIconic != 0 {
		procShowWindow.Call(hwnd, swRestore)
	}

	procSetForeground.Call(hwnd)
}
