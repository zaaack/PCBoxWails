package main

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"PcBoxWails/internal/ipc"
	"PcBoxWails/internal/server"
)

const defaultProxyPort = 9897

type ServerApp struct {
	wsServer        *server.WsServer
	proxyServer     *server.ProxyServer
	downloadManager *DownloadManager
	ipcServer       *ipc.IPCServer
	windowCmd       *exec.Cmd
	mu              sync.Mutex
	topicCallbacks  map[string]chan interface{}
	topicMu         sync.Mutex
	selectedLanIp   string
}

func (a *ServerApp) startup() {
	a.topicCallbacks = make(map[string]chan interface{})
	proxyPort := defaultProxyPort
	if envPort := os.Getenv("PCBOX_PROXY_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 {
			proxyPort = p
		}
	}
	a.proxyServer = server.NewProxyServer()
	a.proxyServer.MountSPA(assets)
	a.proxyServer.SetAPIHandler(a.makeHTTPHandler())
	if err := a.proxyServer.Start(proxyPort); err != nil {
		log.Printf("[PCBox] Proxy server failed to start: %v", err)
	}
	a.downloadManager = NewDownloadManager()
	a.proxyServer.SetCacheDir(a.downloadManager.GetCacheDir())
	a.downloadManager.ResumePendingDownloads(func(id string, progress DownloadProgress) {
		a.emitEvent("download-progress", progress)
	})
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
	if eventName == "ws-response" {
		if m, ok := data.(map[string]interface{}); ok {
			if topicID, ok := m["topicId"].(string); ok {
				a.topicMu.Lock()
				if ch, ok := a.topicCallbacks[topicID]; ok {
					select {
					case ch <- m["data"]:
					default:
					}
				}
				a.topicMu.Unlock()
			}
		}
	}
	if a.ipcServer != nil {
		a.ipcServer.EmitEvent(eventName, data)
	}
	if a.proxyServer != nil {
		a.proxyServer.BroadcastEvent(eventName, data)
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

func (a *ServerApp) GetLocalIps() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var ips []string
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}
	return ips
}

func (a *ServerApp) GetSelectedLanIp() string {
	return a.selectedLanIp
}

func (a *ServerApp) SetSelectedLanIp(ip string) {
	a.selectedLanIp = ip
	if a.proxyServer != nil {
		a.proxyServer.SetBindAddress(ip)
	}
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

func (a *ServerApp) DownloadVideoWithMeta(rawURL string, headers map[string]string, videoName string, sourceKey string, playFlag string, episodeIndex int, vodId string, vodPic string) string {
	if a.downloadManager == nil {
		return ""
	}
	return a.downloadManager.DownloadVideoWithMeta(rawURL, headers, videoName, sourceKey, playFlag, episodeIndex, vodId, vodPic, func(id string, progress DownloadProgress) {
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

func (a *ServerApp) GetDownloadQueue() []DownloadRecord {
	if a.downloadManager == nil {
		return []DownloadRecord{}
	}
	return a.downloadManager.GetDownloadQueue()
}

func (a *ServerApp) CancelDownload(id string) bool {
	if a.downloadManager == nil {
		return false
	}
	return a.downloadManager.CancelDownload(id)
}

func (a *ServerApp) RetryDownload(id string) bool {
	if a.downloadManager == nil {
		return false
	}
	return a.downloadManager.RetryDownload(id, func(id string, progress DownloadProgress) {
		a.emitEvent("download-progress", progress)
	})
}

func (a *ServerApp) ListCachedFilesPaged(page int, pageSize int, keyword string, status string) ([]DownloadRecord, int64) {
	if a.downloadManager == nil {
		return []DownloadRecord{}, 0
	}
	return a.downloadManager.ListCachedFilesPaged(page, pageSize, keyword, status)
}

func (a *ServerApp) DeleteCacheByID(id int) bool {
	if a.downloadManager == nil {
		return false
	}
	return a.downloadManager.DeleteCacheByID(uint(id))
}

func (a *ServerApp) DeleteCacheBatch(ids []int) int {
	if a.downloadManager == nil {
		return 0
	}
	uintIds := make([]uint, len(ids))
	for i, id := range ids {
		uintIds[i] = uint(id)
	}
	return a.downloadManager.DeleteCacheBatch(uintIds)
}

func (a *ServerApp) GetCacheStats() map[string]interface{} {
	if a.downloadManager == nil {
		return map[string]interface{}{"total": 0, "totalSize": 0, "pending": 0}
	}
	return a.downloadManager.GetCacheStats()
}

func (a *ServerApp) SaveCacheProgress(filePath string, progress int, duration int) bool {
	if a.downloadManager == nil {
		return false
	}
	a.downloadManager.SaveCacheProgress(filePath, progress, duration)
	return true
}

func (a *ServerApp) GetCacheProgress(filePath string) map[string]interface{} {
	if a.downloadManager == nil {
		return map[string]interface{}{"progress": 0, "duration": 0}
	}
	return a.downloadManager.GetCacheProgress(filePath)
}

func (a *ServerApp) SavePlayHistory(entry PlayHistoryEntry) bool {
	if a.downloadManager == nil {
		return false
	}
	a.downloadManager.SavePlayHistory(entry)
	return true
}

func (a *ServerApp) GetPlayHistory() []*PlayHistoryEntry {
	if a.downloadManager == nil {
		return []*PlayHistoryEntry{}
	}
	return a.downloadManager.GetPlayHistory()
}

func (a *ServerApp) FindNextCachedEpisode(sourceKey string, playFlag string, episodeIndex int) *DownloadRecord {
	if a.downloadManager == nil {
		return nil
	}
	return a.downloadManager.FindNextCachedEpisode(sourceKey, playFlag, episodeIndex)
}

func (a *ServerApp) FindDownloadRecordByFilePath(filePath string) *DownloadRecord {
	if a.downloadManager == nil {
		return nil
	}
	return a.downloadManager.FindDownloadRecordByFilePath(filePath)
}

func (a *ServerApp) SendTopicMessageHTTP(clientID string, code int, data map[string]interface{}, topicID string) interface{} {
	ch := make(chan interface{}, 1)
	a.topicMu.Lock()
	a.topicCallbacks[topicID] = ch
	a.topicMu.Unlock()

	defer func() {
		a.topicMu.Lock()
		delete(a.topicCallbacks, topicID)
		a.topicMu.Unlock()
	}()

	a.SendMessage(clientID, code, data)

	select {
	case result := <-ch:
		return result
	case <-time.After(60 * time.Second):
		return nil
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func (a *ServerApp) makeHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/StartWsServer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var args []int
		readJSON(r, &args)
		writeJSON(w, a.StartWsServer(args[0]))
	})

	mux.HandleFunc("/StopWsServer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		writeJSON(w, a.StopWsServer())
	})

	mux.HandleFunc("/GetWsServerStatus", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetWsServerStatus())
	})

	mux.HandleFunc("/GetLocalIp", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetLocalIp())
	})

	mux.HandleFunc("/GetLocalIps", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetLocalIps())
	})

	mux.HandleFunc("/GetSelectedLanIp", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetSelectedLanIp())
	})

	mux.HandleFunc("/SetSelectedLanIp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var ip string
		if err := readJSON(r, &ip); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		a.SetSelectedLanIp(ip)
		writeJSON(w, true)
	})

	mux.HandleFunc("/GetClients", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetClients())
	})

	mux.HandleFunc("/SendMessage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			ClientID string      `json:"clientId"`
			Code     int         `json:"code"`
			Data     interface{} `json:"data"`
		}
		readJSON(r, &p)
		writeJSON(w, a.SendMessage(p.ClientID, p.Code, p.Data))
	})

	mux.HandleFunc("/CreateProxySession", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		}
		readJSON(r, &p)
		writeJSON(w, a.CreateProxySession(p.URL, p.Headers))
	})

	mux.HandleFunc("/GetProxyPort", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetProxyPort())
	})

	mux.HandleFunc("/SetCacheDir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var dir string
		readJSON(r, &dir)
		a.SetCacheDir(dir)
		writeJSON(w, true)
	})

	mux.HandleFunc("/GetCacheDir", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetCacheDir())
	})

	mux.HandleFunc("/SelectCacheDir", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		writeJSON(w, "")
	})

	mux.HandleFunc("/DownloadVideo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			URL       string            `json:"url"`
			Headers   map[string]string `json:"headers"`
			VideoName string            `json:"videoName"`
		}
		readJSON(r, &p)
		writeJSON(w, a.DownloadVideo(p.URL, p.Headers, p.VideoName))
	})

	mux.HandleFunc("/DownloadVideoWithMeta", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			URL          string            `json:"url"`
			Headers      map[string]string `json:"headers"`
			VideoName    string            `json:"videoName"`
			SourceKey    string            `json:"sourceKey"`
			PlayFlag     string            `json:"playFlag"`
			EpisodeIndex int               `json:"episodeIndex"`
			VodId        string            `json:"vodId"`
			VodPic       string            `json:"vodPic"`
		}
		if err := readJSON(r, &p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, a.DownloadVideoWithMeta(p.URL, p.Headers, p.VideoName, p.SourceKey, p.PlayFlag, p.EpisodeIndex, p.VodId, p.VodPic))
	})

	mux.HandleFunc("/GetCachedFile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var rawURL string
		readJSON(r, &rawURL)
		writeJSON(w, a.GetCachedFile(rawURL))
	})

	mux.HandleFunc("/GetDownloadProgress", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var id string
		readJSON(r, &id)
		writeJSON(w, a.GetDownloadProgress(id))
	})

	mux.HandleFunc("/ListCachedFiles", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.ListCachedFiles())
	})

	mux.HandleFunc("/DeleteCachedFile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var rawURL string
		readJSON(r, &rawURL)
		writeJSON(w, a.DeleteCachedFile(rawURL))
	})

	mux.HandleFunc("/GetDownloadQueue", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetDownloadQueue())
	})

	mux.HandleFunc("/CancelDownload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var id string
		readJSON(r, &id)
		writeJSON(w, a.CancelDownload(id))
	})

	mux.HandleFunc("/RetryDownload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var id string
		readJSON(r, &id)
		writeJSON(w, a.RetryDownload(id))
	})

	mux.HandleFunc("/ListCachedFilesPaged", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			Page     int    `json:"page"`
			PageSize int    `json:"pageSize"`
			Keyword  string `json:"keyword"`
			Status   string `json:"status"`
		}
		readJSON(r, &p)
		if p.Page <= 0 {
			p.Page = 1
		}
		if p.PageSize <= 0 {
			p.PageSize = 20
		}
		records, total := a.ListCachedFilesPaged(p.Page, p.PageSize, p.Keyword, p.Status)
		writeJSON(w, map[string]interface{}{
			"records":  records,
			"total":    total,
			"page":     p.Page,
			"pageSize": p.PageSize,
		})
	})

	mux.HandleFunc("/DeleteCacheByID", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var id int
		readJSON(r, &id)
		writeJSON(w, a.DeleteCacheByID(id))
	})

	mux.HandleFunc("/DeleteCacheBatch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var ids []int
		readJSON(r, &ids)
		writeJSON(w, a.DeleteCacheBatch(ids))
	})

	mux.HandleFunc("/GetCacheStats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetCacheStats())
	})

	mux.HandleFunc("/SaveCacheProgress", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			FilePath string `json:"filePath"`
			Progress int    `json:"progress"`
			Duration int    `json:"duration"`
		}
		if err := readJSON(r, &p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, a.SaveCacheProgress(p.FilePath, p.Progress, p.Duration))
	})

	mux.HandleFunc("/GetCacheProgress", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var filePath string
		if err := readJSON(r, &filePath); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, a.GetCacheProgress(filePath))
	})

	mux.HandleFunc("/SavePlayHistory", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var entry PlayHistoryEntry
		if err := readJSON(r, &entry); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, a.SavePlayHistory(entry))
	})

	mux.HandleFunc("/GetPlayHistory", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, a.GetPlayHistory())
	})

	mux.HandleFunc("/FindNextCachedEpisode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			SourceKey    string `json:"sourceKey"`
			PlayFlag     string `json:"playFlag"`
			EpisodeIndex int    `json:"episodeIndex"`
		}
		if err := readJSON(r, &p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, a.FindNextCachedEpisode(p.SourceKey, p.PlayFlag, p.EpisodeIndex))
	})

	mux.HandleFunc("/FindDownloadRecordByFilePath", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var filePath string
		if err := readJSON(r, &filePath); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, a.FindDownloadRecordByFilePath(filePath))
	})

	mux.HandleFunc("/SendTopicMessage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" { http.Error(w, "POST required", 405); return }
		var p struct {
			ClientID string                 `json:"clientId"`
			Code     int                    `json:"code"`
			Data     map[string]interface{} `json:"data"`
			TopicID  string                 `json:"topicId"`
		}
		if err := readJSON(r, &p); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		result := a.SendTopicMessageHTTP(p.ClientID, p.Code, p.Data, p.TopicID)
		writeJSON(w, result)
	})

	return mux
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

	ipcSrv.RegisterMethod("GetLocalIps", func(args json.RawMessage) (interface{}, error) {
		return srv.GetLocalIps(), nil
	})

	ipcSrv.RegisterMethod("GetSelectedLanIp", func(args json.RawMessage) (interface{}, error) {
		return srv.GetSelectedLanIp(), nil
	})

	ipcSrv.RegisterMethod("SetSelectedLanIp", func(args json.RawMessage) (interface{}, error) {
		var ip string
		if err := json.Unmarshal(args, &ip); err != nil {
			return nil, err
		}
		srv.SetSelectedLanIp(ip)
		return true, nil
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

	ipcSrv.RegisterMethod("DownloadVideoWithMeta", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			URL          string            `json:"url"`
			Headers      map[string]string `json:"headers"`
			VideoName    string            `json:"videoName"`
			SourceKey    string            `json:"sourceKey"`
			PlayFlag     string            `json:"playFlag"`
			EpisodeIndex int               `json:"episodeIndex"`
			VodId        string            `json:"vodId"`
			VodPic       string            `json:"vodPic"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		return srv.DownloadVideoWithMeta(p.URL, p.Headers, p.VideoName, p.SourceKey, p.PlayFlag, p.EpisodeIndex, p.VodId, p.VodPic), nil
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

	ipcSrv.RegisterMethod("GetDownloadQueue", func(args json.RawMessage) (interface{}, error) {
		return srv.GetDownloadQueue(), nil
	})

	ipcSrv.RegisterMethod("CancelDownload", func(args json.RawMessage) (interface{}, error) {
		var id string
		if err := json.Unmarshal(args, &id); err != nil {
			return nil, err
		}
		return srv.CancelDownload(id), nil
	})

	ipcSrv.RegisterMethod("RetryDownload", func(args json.RawMessage) (interface{}, error) {
		var id string
		if err := json.Unmarshal(args, &id); err != nil {
			return nil, err
		}
		return srv.RetryDownload(id), nil
	})

	ipcSrv.RegisterMethod("ListCachedFilesPaged", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			Page     int    `json:"page"`
			PageSize int    `json:"pageSize"`
			Keyword  string `json:"keyword"`
			Status   string `json:"status"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		if p.Page <= 0 {
			p.Page = 1
		}
		if p.PageSize <= 0 {
			p.PageSize = 20
		}
		records, total := srv.ListCachedFilesPaged(p.Page, p.PageSize, p.Keyword, p.Status)
		return map[string]interface{}{
			"records":  records,
			"total":    total,
			"page":     p.Page,
			"pageSize": p.PageSize,
		}, nil
	})

	ipcSrv.RegisterMethod("DeleteCacheByID", func(args json.RawMessage) (interface{}, error) {
		var id int
		if err := json.Unmarshal(args, &id); err != nil {
			return nil, err
		}
		return srv.DeleteCacheByID(id), nil
	})

	ipcSrv.RegisterMethod("DeleteCacheBatch", func(args json.RawMessage) (interface{}, error) {
		var ids []int
		if err := json.Unmarshal(args, &ids); err != nil {
			return nil, err
		}
		return srv.DeleteCacheBatch(ids), nil
	})

	ipcSrv.RegisterMethod("GetCacheStats", func(args json.RawMessage) (interface{}, error) {
		return srv.GetCacheStats(), nil
	})

	ipcSrv.RegisterMethod("SaveCacheProgress", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			FilePath string `json:"filePath"`
			Progress int    `json:"progress"`
			Duration int    `json:"duration"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		return srv.SaveCacheProgress(p.FilePath, p.Progress, p.Duration), nil
	})

	ipcSrv.RegisterMethod("GetCacheProgress", func(args json.RawMessage) (interface{}, error) {
		var filePath string
		if err := json.Unmarshal(args, &filePath); err != nil {
			return nil, err
		}
		return srv.GetCacheProgress(filePath), nil
	})

	ipcSrv.RegisterMethod("SavePlayHistory", func(args json.RawMessage) (interface{}, error) {
		var entry PlayHistoryEntry
		if err := json.Unmarshal(args, &entry); err != nil {
			return nil, err
		}
		return srv.SavePlayHistory(entry), nil
	})

	ipcSrv.RegisterMethod("GetPlayHistory", func(args json.RawMessage) (interface{}, error) {
		return srv.GetPlayHistory(), nil
	})

	ipcSrv.RegisterMethod("FindNextCachedEpisode", func(args json.RawMessage) (interface{}, error) {
		var p struct {
			SourceKey    string `json:"sourceKey"`
			PlayFlag     string `json:"playFlag"`
			EpisodeIndex int    `json:"episodeIndex"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
		return srv.FindNextCachedEpisode(p.SourceKey, p.PlayFlag, p.EpisodeIndex), nil
	})

	ipcSrv.RegisterMethod("FindDownloadRecordByFilePath", func(args json.RawMessage) (interface{}, error) {
		var filePath string
		if err := json.Unmarshal(args, &filePath); err != nil {
			return nil, err
		}
		return srv.FindDownloadRecordByFilePath(filePath), nil
	})
}
