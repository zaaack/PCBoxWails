package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"PcBoxWails/internal/ipc"
	"PcBoxWails/internal/server"
	"PcBoxWails/internal/tray"
)

var (
	srv       *ServerApp
	ipcSrv    *ipc.IPCServer
	windowCmd *exec.Cmd
)

type ServerApp struct {
	wsServer    *server.WsServer
	proxyServer *server.ProxyServer
	mu          sync.Mutex
}

func (a *ServerApp) startup() {
	a.proxyServer = server.NewProxyServer()
	a.proxyServer.Start()
	log.Println("[PCBox] Proxy server started")
}

func (a *ServerApp) shutdown() {
	if a.wsServer != nil {
		a.wsServer.Stop()
	}
	if a.proxyServer != nil {
		a.proxyServer.Stop()
	}
}

func (a *ServerApp) emitEvent(eventName string, data interface{}) {
	if ipcSrv != nil {
		ipcSrv.EmitEvent(eventName, data)
	}
}

func (a *ServerApp) StartWsServer(port int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wsServer != nil {
		a.wsServer.Stop()
	}
	a.wsServer = server.NewWsServer(a.emitEvent)
	return a.wsServer.Start(port)
}

func (a *ServerApp) StopWsServer() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wsServer != nil {
		a.wsServer.Stop()
		a.wsServer = nil
	}
	return true
}

func (a *ServerApp) GetWsServerStatus() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wsServer == nil {
		return map[string]interface{}{"running": false, "port": 0}
	}
	return a.wsServer.GetStatus()
}

func (a *ServerApp) GetLocalIp() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func (a *ServerApp) GetClients() []server.ClientInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wsServer == nil {
		return []server.ClientInfo{}
	}
	return a.wsServer.GetClients()
}

func (a *ServerApp) SendMessage(clientID string, code int, data interface{}) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wsServer == nil {
		return false
	}
	return a.wsServer.SendMessage(clientID, code, data)
}

func (a *ServerApp) CreateProxySession(url string, headers map[string]string) string {
	if a.proxyServer == nil {
		return ""
	}
	return a.proxyServer.CreateSession(url, headers)
}

func (a *ServerApp) GetProxyPort() int {
	if a.proxyServer == nil {
		return 0
	}
	return a.proxyServer.Port()
}

func main() {
	srv = &ServerApp{}
	srv.startup()
	defer srv.shutdown()

	ipcSrv = ipc.NewIPCServer()
	registerIPCMethods()

	go func() {
		if err := ipcSrv.Start(9899); err != nil {
			log.Fatalf("[IPC] Server failed: %v", err)
		}
	}()

	loadIcon := func() []byte {
		exe, _ := os.Executable()
		pngPath := filepath.Join(filepath.Dir(exe), "appicon.png")
		data, err := os.ReadFile(pngPath)
		if err != nil {
			log.Printf("[Tray] Failed to load icon: %v", err)
			return nil
		}
		return data
	}

	iconData := loadIcon()

	t := tray.New()
	if iconData != nil {
		t.SetIcon(iconData)
	}
	t.SetTooltip("PCBox Server")

	menu := tray.NewMenu()
	menu.Add("显示窗口", func() { showWindow() })
	menu.AddSeparator()
	menu.Add("退出", func() {
		t.Remove()
		os.Exit(0)
	})
	t.SetMenu(menu)
	t.OnDoubleClick(func() { showWindow() })

	t.Show()
	if err := t.Run(); err != nil {
		log.Fatalf("[Tray] Run error: %v", err)
	}
}

func showWindow() {
	if windowCmd != nil && windowCmd.Process != nil {
		log.Println("[Server] Window already running")
		return
	}

	exe, err := os.Executable()
	if err != nil {
		log.Printf("[Server] Failed to get exe path: %v", err)
		return
	}

	dir := filepath.Dir(exe)
	windowExe := filepath.Join(dir, "pcbox-window.exe")
	if _, err := os.Stat(windowExe); os.IsNotExist(err) {
		windowExe = exe
	}

	windowCmd = exec.Command(windowExe, "--mode=window", "--ipc-port=9899")
	windowCmd.Stdout = os.Stdout
	windowCmd.Stderr = os.Stderr

	if err := windowCmd.Start(); err != nil {
		log.Printf("[Server] Failed to start window: %v", err)
		windowCmd = nil
		return
	}

	go func() {
		windowCmd.Wait()
		windowCmd = nil
		log.Println("[Server] Window process exited")
	}()
}

func registerIPCMethods() {
	ipcSrv.RegisterMethod("StartWsServer", func(args json.RawMessage) (interface{}, error) {
		var port int
		if err := json.Unmarshal(args, &port); err != nil {
			return nil, err
		}
		return srv.StartWsServer(port), nil
	})

	ipcSrv.RegisterMethod("StopWsServer", func(args json.RawMessage) (interface{}, error) {
		return srv.StopWsServer(), nil
	})

	ipcSrv.RegisterMethod("GetWsServerStatus", func(args json.RawMessage) (interface{}, error) {
		return srv.GetWsServerStatus(), nil
	})

	ipcSrv.RegisterMethod("GetLocalIp", func(args json.RawMessage) (interface{}, error) {
		return srv.GetLocalIp(), nil
	})

	ipcSrv.RegisterMethod("GetClients", func(args json.RawMessage) (interface{}, error) {
		return srv.GetClients(), nil
	})

	ipcSrv.RegisterMethod("SendMessage", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			ClientID string      `json:"clientId"`
			Code     int         `json:"code"`
			Data     interface{} `json:"data"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		return srv.SendMessage(p.ClientID, p.Code, p.Data), nil
	})

	ipcSrv.RegisterMethod("CreateProxySession", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		return srv.CreateProxySession(p.URL, p.Headers), nil
	})

	ipcSrv.RegisterMethod("GetProxyPort", func(args json.RawMessage) (interface{}, error) {
		return srv.GetProxyPort(), nil
	})

	_ = fmt.Sprintf
	_ = time.Now
}
