package main

import (
	"context"
	"log"
	"net"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ClientInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ConnectedAt int64  `json:"connectedAt"`
}

type App struct {
	ctx             context.Context
	wsServer        *WsServer
	proxyServer     *ProxyServer
	downloadManager *DownloadManager
	mu              sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.proxyServer = NewProxyServer()
	a.proxyServer.Start()
	a.downloadManager = NewDownloadManager()
	log.Println("[PCBox] Proxy server started")
}

func (a *App) shutdown(ctx context.Context) {
	if a.wsServer != nil {
		a.wsServer.Stop()
	}
	if a.proxyServer != nil {
		a.proxyServer.Stop()
	}
}

func (a *App) emitEvent(eventName string, data interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventName, data)
	}
}

func (a *App) StartWsServer(port int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.wsServer != nil {
		a.wsServer.Stop()
	}

	a.wsServer = NewWsServer(a)
	return a.wsServer.Start(port)
}

func (a *App) StopWsServer() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.wsServer != nil {
		a.wsServer.Stop()
		a.wsServer = nil
	}
	return true
}

func (a *App) GetWsServerStatus() map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.wsServer == nil {
		return map[string]interface{}{"running": false, "port": 0}
	}
	return a.wsServer.GetStatus()
}

func (a *App) GetLocalIp() string {
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

func (a *App) GetClients() []ClientInfo {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.wsServer == nil {
		return []ClientInfo{}
	}
	return a.wsServer.clientManager.GetAll()
}

func (a *App) SendMessage(clientID string, code int, data interface{}) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.wsServer == nil {
		return false
	}
	return a.wsServer.SendMessage(clientID, code, data)
}

func (a *App) CreateProxySession(url string, headers map[string]string) string {
	if a.proxyServer == nil {
		return ""
	}
	return a.proxyServer.CreateSession(url, headers)
}

func (a *App) SetCacheDir(dir string) {
	if a.downloadManager == nil {
		return
	}
	a.downloadManager.SetCacheDir(dir)
}

func (a *App) GetCacheDir() string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.GetCacheDir()
}

func (a *App) SelectCacheDir() string {
	if a.ctx == nil {
		return ""
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Cache Directory",
	})
	if err != nil || dir == "" {
		return ""
	}
	a.downloadManager.SetCacheDir(dir)
	return dir
}

func (a *App) DownloadVideo(rawURL string, headers map[string]string, videoName string) string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.DownloadVideo(rawURL, headers, videoName, func(id string, progress DownloadProgress) {
		a.emitEvent("download-progress", progress)
	})
}

func (a *App) GetCachedFile(rawURL string) string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.GetCachedFile(rawURL)
}

func (a *App) GetDownloadProgress(id string) *DownloadProgress {
	if a.downloadManager == nil {
		return nil
	}
	return a.downloadManager.GetDownloadProgress(id)
}

func (a *App) ListCachedFiles() []*CachedVideo {
	if a.downloadManager == nil {
		return []*CachedVideo{}
	}
	return a.downloadManager.ListCachedFiles()
}

func (a *App) DeleteCachedFile(rawURL string) bool {
	if a.downloadManager == nil {
		return false
	}
	return a.downloadManager.DeleteCachedFile(rawURL)
}
