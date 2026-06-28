package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"PcBoxWails/internal/ipc"
)

//go:embed all:frontend/dist
var assets embed.FS

type WindowApp struct {
	ipcClient *ipc.IPCClient
}

func NewWindowApp(ipcPort int) *WindowApp {
	return &WindowApp{
		ipcClient: ipc.NewIPCClient(ipcPort),
	}
}

func (a *WindowApp) StartWsServer(port int) bool {
	result, err := a.ipcClient.Call("StartWsServer", port)
	if err != nil {
		log.Printf("[Window] StartWsServer error: %v", err)
		return false
	}
	return toBool(result)
}

func (a *WindowApp) StopWsServer() bool {
	result, err := a.ipcClient.Call("StopWsServer", nil)
	if err != nil {
		log.Printf("[Window] StopWsServer error: %v", err)
		return false
	}
	return toBool(result)
}

func (a *WindowApp) GetWsServerStatus() map[string]interface{} {
	result, err := a.ipcClient.Call("GetWsServerStatus", nil)
	if err != nil {
		log.Printf("[Window] GetWsServerStatus error: %v", err)
		return map[string]interface{}{"running": false, "port": 0}
	}
	return toMap(result)
}

func (a *WindowApp) GetLocalIp() string {
	result, err := a.ipcClient.Call("GetLocalIp", nil)
	if err != nil {
		log.Printf("[Window] GetLocalIp error: %v", err)
		return "127.0.0.1"
	}
	return toString(result)
}

func (a *WindowApp) GetClients() []map[string]interface{} {
	result, err := a.ipcClient.Call("GetClients", nil)
	if err != nil {
		log.Printf("[Window] GetClients error: %v", err)
		return []map[string]interface{}{}
	}
	return toSlice(result)
}

func (a *WindowApp) SendMessage(clientId string, code int, data interface{}) bool {
	result, err := a.ipcClient.Call("SendMessage", map[string]interface{}{
		"clientId": clientId,
		"code":     code,
		"data":     data,
	})
	if err != nil {
		log.Printf("[Window] SendMessage error: %v", err)
		return false
	}
	return toBool(result)
}

func (a *WindowApp) CreateProxySession(url string, headers map[string]string) string {
	result, err := a.ipcClient.Call("CreateProxySession", map[string]interface{}{
		"url":     url,
		"headers": headers,
	})
	if err != nil {
		log.Printf("[Window] CreateProxySession error: %v", err)
		return ""
	}
	return toString(result)
}

func (a *WindowApp) GetProxyPort() int {
	result, err := a.ipcClient.Call("GetProxyPort", nil)
	if err != nil {
		log.Printf("[Window] GetProxyPort error: %v", err)
		return 0
	}
	return toInt(result)
}

func toBool(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func toInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	if i, ok := v.(int); ok {
		return i
	}
	return 0
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toMap(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

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

func toSlice(v interface{}) []map[string]interface{} {
	if arr, ok := v.([]interface{}); ok {
		result := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				result = append(result, m)
			}
		}
		return result
	}
	return []map[string]interface{}{}
}

func main() {
	mode := flag.String("mode", "window", "Run mode: window or standalone")
	ipcPort := flag.Int("ipc-port", 9899, "IPC server port")
	flag.Parse()

	if *mode == "standalone" {
		log.Fatal("Standalone mode not yet implemented in window process")
	}

	app := NewWindowApp(*ipcPort)

	if err := app.ipcClient.Connect(); err != nil {
		log.Fatalf("[Window] Failed to connect to server: %v", err)
	}
	defer app.ipcClient.Close()

	_ = json.Marshal

	err := wails.Run(&options.App{
		Title:     "PCBox",
		Width:     800,
		Height:    500,
		MinWidth:  600,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 1},
		Bind: []interface{}{
			app,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.pcbox.app",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				log.Printf("Second instance launched: %v", secondInstanceData.Args)
			},
		},
	})

	if err != nil {
		fmt.Println("Error:", err.Error())
	}

	os.Exit(0)
}
