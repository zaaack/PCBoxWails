package main

import (
	"encoding/json"
	"log"
	"net"
	"os/exec"
	"sync"

	"PcBoxWails/internal/ipc"
	"PcBoxWails/internal/server"
)

type ServerApp struct {
	wsServer        *server.WsServer
	proxyServer     *server.ProxyServer
	downloadManager *DownloadManager
	ipcServer       *ipc.IPCServer
	windowCmd       *exec.Cmd
	mu              sync.Mutex
}

func (a *ServerApp) startup() {
	a.proxyServer = server.NewProxyServer()
	a.proxyServer.Start()
	a.downloadManager = NewDownloadManager()
	a.wsServer = server.NewWsServer(a.emitEvent)
	a.wsServer.Start(9898)
	log.Println("[PCBox] WebSocket server started on port 9898")
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
	if a.ipcServer != nil {
		a.ipcServer.EmitEvent(eventName, data)
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

func (a *ServerApp) SetCacheDir(dir string) {
	if a.downloadManager == nil {
		return
	}
	a.downloadManager.SetCacheDir(dir)
}

func (a *ServerApp) GetCacheDir() string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.GetCacheDir()
}

func (a *ServerApp) DownloadVideo(rawURL string, headers map[string]string, videoName string) string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.DownloadVideo(rawURL, headers, videoName, func(id string, progress DownloadProgress) {
		a.emitEvent("download-progress", progress)
	})
}

func (a *ServerApp) GetCachedFile(rawURL string) string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.GetCachedFile(rawURL)
}

func (a *ServerApp) GetDownloadProgress(id string) *DownloadProgress {
	if a.downloadManager == nil {
		return nil
	}
	return a.downloadManager.GetDownloadProgress(id)
}

func (a *ServerApp) ListCachedFiles() []*CachedVideo {
	if a.downloadManager == nil {
		return []*CachedVideo{}
	}
	return a.downloadManager.ListCachedFiles()
}

func (a *ServerApp) DeleteCachedFile(rawURL string) bool {
	if a.downloadManager == nil {
		return false
	}
	return a.downloadManager.DeleteCachedFile(rawURL)
}

func registerIPCMethods(srv *ServerApp, ipcSrv *ipc.IPCServer) {
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
		status := srv.GetWsServerStatus()
		log.Printf("[IPC] GetWsServerStatus: %v", status)
		return status, nil
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

	ipcSrv.RegisterMethod("SetCacheDir", func(args json.RawMessage) (interface{}, error) {
		var dir string
		if err := json.Unmarshal(args, &dir); err != nil {
			return nil, err
		}
		srv.SetCacheDir(dir)
		return true, nil
	})

	ipcSrv.RegisterMethod("GetCacheDir", func(args json.RawMessage) (interface{}, error) {
		return srv.GetCacheDir(), nil
	})

	ipcSrv.RegisterMethod("DownloadVideo", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			URL       string            `json:"url"`
			Headers   map[string]string `json:"headers"`
			VideoName string            `json:"videoName"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		return srv.DownloadVideo(p.URL, p.Headers, p.VideoName), nil
	})

	ipcSrv.RegisterMethod("GetCachedFile", func(args json.RawMessage) (interface{}, error) {
		var rawURL string
		if err := json.Unmarshal(args, &rawURL); err != nil {
			return nil, err
		}
		return srv.GetCachedFile(rawURL), nil
	})

	ipcSrv.RegisterMethod("GetDownloadProgress", func(args json.RawMessage) (interface{}, error) {
		var id string
		if err := json.Unmarshal(args, &id); err != nil {
			return nil, err
		}
		return srv.GetDownloadProgress(id), nil
	})

	ipcSrv.RegisterMethod("ListCachedFiles", func(args json.RawMessage) (interface{}, error) {
		return srv.ListCachedFiles(), nil
	})

	ipcSrv.RegisterMethod("DeleteCachedFile", func(args json.RawMessage) (interface{}, error) {
		var rawURL string
		if err := json.Unmarshal(args, &rawURL); err != nil {
			return nil, err
		}
		return srv.DeleteCachedFile(rawURL), nil
	})
}
