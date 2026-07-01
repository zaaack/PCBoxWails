//go:build windows

package main

import (
	"time"
	"unsafe"
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
