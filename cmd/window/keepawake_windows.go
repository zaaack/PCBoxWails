//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	user32                 = syscall.NewLazyDLL("user32.dll")
	procSetThreadExecState = kernel32.NewProc("SetThreadExecutionState")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
	procSetCursorPos       = user32.NewProc("SetCursorPos")
)

type POINT struct{ X, Y int32 }

var keepAwakeStop chan struct{}

func (a *WindowApp) SetKeepScreenOn(active bool) {
	if active {
		if keepAwakeStop != nil {
			return
		}
		keepAwakeStop = make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					var pos POINT
					procGetCursorPos.Call(uintptr(unsafe.Pointer(&pos)))
					procSetCursorPos.Call(uintptr(pos.X+100), uintptr(pos.Y))
					time.Sleep(50 * time.Millisecond)
					procSetCursorPos.Call(uintptr(pos.X), uintptr(pos.Y))
				case <-keepAwakeStop:
					return
				}
			}
		}()
	} else {
		if keepAwakeStop != nil {
			close(keepAwakeStop)
			keepAwakeStop = nil
		}
	}
}
