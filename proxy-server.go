package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

type ProxySession struct {
	URL     string
	Headers map[string]string
}

type ProxyServer struct {
	sessions map[string]*ProxySession
	server   *http.Server
	port     int
	mu       sync.RWMutex
}

func NewProxyServer() *ProxyServer {
	return &ProxyServer{
		sessions: make(map[string]*ProxySession),
	}
}

func (p *ProxyServer) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}

	p.port = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy", p.handleProxy)
	mux.HandleFunc("/local", p.handleLocal)

	p.server = &http.Server{
		Handler: mux,
	}

	go func() {
		if err := p.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[Proxy] Server error: %v", err)
		}
	}()

	log.Printf("[Proxy] Server started on port %d", p.port)
	return nil
}

func (p *ProxyServer) Stop() {
	if p.server != nil {
		p.server.Close()
		p.server = nil
	}
	p.mu.Lock()
	p.sessions = make(map[string]*ProxySession)
	p.mu.Unlock()
}

func (p *ProxyServer) Port() int {
	return p.port
}

func (p *ProxyServer) handleLocal(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("u")
	if filePath == "" {
		http.Error(w, "Missing u parameter", http.StatusBadRequest)
		return
	}

	if strings.HasSuffix(filePath, ".m3u8") || strings.Contains(filePath, ".m3u8") {
		content, err := os.ReadFile(filePath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		dir := filePath
		if idx := strings.LastIndex(filePath, "\\"); idx >= 0 {
			dir = filePath[:idx+1]
		} else if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
			dir = filePath[:idx+1]
		}
		lines := strings.Split(string(content), "\n")
		var result []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				result = append(result, line)
				continue
			}
			var resolved string
			if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
				resolved = trimmed
			} else if len(trimmed) >= 2 && trimmed[1] == ':' {
				resolved = trimmed
			} else if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
				resolved = trimmed
			} else {
				resolved = dir + trimmed
			}
			proxyURL := fmt.Sprintf("http://127.0.0.1:%d/local?u=%s", p.port, url.QueryEscape(resolved))
			result = append(result, proxyURL)
		}
		w.Header().Set("Content-Type", "application/x-mpegURL")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write([]byte(strings.Join(result, "\n")))
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")

	f, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "File stat error", http.StatusInternalServerError)
		return
	}

	ext := filePath[strings.LastIndex(filePath, "."):]
	contentType := "application/octet-stream"
	switch ext {
	case ".ts":
		contentType = "video/mp2t"
	case ".m3u8":
		contentType = "application/x-mpegURL"
	case ".mp4":
		contentType = "video/mp4"
	}
	w.Header().Set("Content-Type", contentType)
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), f)
}

func (p *ProxyServer) CreateSession(targetURL string, headers map[string]string) string {
	id := fmt.Sprintf("%d", time.Now().UnixNano())

	p.mu.Lock()
	p.sessions[id] = &ProxySession{
		URL:     targetURL,
		Headers: headers,
	}
	p.mu.Unlock()

	return fmt.Sprintf("http://127.0.0.1:%d/proxy?id=%s&u=%s", p.port, id, url.QueryEscape(targetURL))
}

func (p *ProxyServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("id")
	targetURLStr := r.URL.Query().Get("u")

	if sessionID == "" || targetURLStr == "" {
		http.Error(w, "Missing id or u parameter", http.StatusBadRequest)
		return
	}

	p.mu.RLock()
	session, ok := p.sessions[sessionID]
	p.mu.RUnlock()

	if !ok {
		http.Error(w, "Session expired", http.StatusNotFound)
		return
	}

	forwardHeaders := make(http.Header)
	for k, v := range session.Headers {
		forwardHeaders.Set(k, v)
	}
	if _, ok := forwardHeaders["User-Agent"]; !ok {
		forwardHeaders.Set("User-Agent", defaultUA)
	}
	forwardHeaders.Set("Accept", "*/*")
	forwardHeaders.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	proxyReq, err := http.NewRequest("GET", targetURLStr, nil)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	proxyReq.Header = forwardHeaders

	client := &http.Client{}
	proxyRes, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("[Proxy] Error: %v", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	defer proxyRes.Body.Close()

	skipHeaders := map[string]bool{
		"transfer-encoding": true,
		"connection":        true,
		"keep-alive":        true,
	}

	contentType := proxyRes.Header.Get("Content-Type")
	isM3U8 := strings.Contains(contentType, "mpegurl") || strings.Contains(contentType, "x-mpegurl") ||
		strings.HasSuffix(targetURLStr, ".m3u8") || strings.Contains(targetURLStr, ".m3u8")

	var body []byte
	if isM3U8 {
		var err error
		body, err = io.ReadAll(proxyRes.Body)
		if err != nil {
			log.Printf("[Proxy] Error reading m3u8: %v", err)
			return
		}
	}

	for k, vv := range proxyRes.Header {
		if skipHeaders[strings.ToLower(k)] {
			continue
		}
		if isM3U8 && strings.ToLower(k) == "content-length" {
			continue
		}
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "*")

	if isM3U8 {
		rewritten := p.rewriteM3U8(string(body), sessionID, targetURLStr)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(rewritten)))
		w.WriteHeader(proxyRes.StatusCode)
		w.Write([]byte(rewritten))
	} else {
		w.WriteHeader(proxyRes.StatusCode)
		io.Copy(w, proxyRes.Body)
	}
}

func (p *ProxyServer) rewriteM3U8(content string, sessionID string, baseM3U8URL string) string {
	baseURL := baseM3U8URL
	if idx := strings.LastIndex(baseM3U8URL, "/"); idx >= 0 {
		baseURL = baseM3U8URL[:idx+1]
	}

	lines := strings.Split(content, "\n")
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}

		var resolvedURL string
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			resolvedURL = trimmed
		} else if strings.HasPrefix(trimmed, "/") {
			parsedBase, err := url.Parse(baseM3U8URL)
			if err == nil {
				resolvedURL = fmt.Sprintf("%s://%s%s", parsedBase.Scheme, parsedBase.Host, trimmed)
			} else {
				resolvedURL = trimmed
			}
		} else {
			resolvedURL = baseURL + trimmed
		}

		proxyURL := fmt.Sprintf("http://127.0.0.1:%d/proxy?id=%s&u=%s", p.port, sessionID, url.QueryEscape(resolvedURL))
		result = append(result, proxyURL)
	}

	return strings.Join(result, "\n")
}
