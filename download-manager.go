package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const defaultUA2 = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

type CachedVideo struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	VideoName string `json:"videoName"`
	FilePath  string `json:"filePath"`
	IsHLS     bool   `json:"isHLS"`
	Size      int64  `json:"size"`
}

type DownloadProgress struct {
	ID       string  `json:"id"`
	Progress float64 `json:"progress"`
	Status   string  `json:"status"` // "downloading", "completed", "failed"
	Error    string  `json:"error,omitempty"`
}

type CacheIndex struct {
	Videos map[string]*CachedVideo `json:"videos"`
}

type DownloadManager struct {
	cacheDir    string
	index       *CacheIndex
	downloads   map[string]*DownloadProgress
	indexMu     sync.RWMutex
	downloadsMu sync.RWMutex
}

func NewDownloadManager() *DownloadManager {
	dm := &DownloadManager{
		downloads: make(map[string]*DownloadProgress),
		index:     &CacheIndex{Videos: make(map[string]*CachedVideo)},
	}
	dm.cacheDir = dm.defaultCacheDir()
	dm.loadIndex()
	return dm
}

func (dm *DownloadManager) defaultCacheDir() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "cache")
	}
	return filepath.Join(filepath.Dir(exe), "cache")
}

func (dm *DownloadManager) SetCacheDir(dir string) {
	dm.indexMu.Lock()
	defer dm.indexMu.Unlock()
	dm.cacheDir = dir
	os.MkdirAll(dir, 0755)
	dm.loadIndex()
}

func (dm *DownloadManager) GetCacheDir() string {
	dm.indexMu.RLock()
	defer dm.indexMu.RUnlock()
	return dm.cacheDir
}

func (dm *DownloadManager) GetCacheIndexFilePath() string {
	return filepath.Join(dm.cacheDir, "cache-index.json")
}

func (dm *DownloadManager) loadIndex() {
	data, err := os.ReadFile(dm.GetCacheIndexFilePath())
	if err != nil {
		return
	}
	var idx CacheIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		log.Printf("[Cache] Failed to load index: %v", err)
		return
	}
	if idx.Videos == nil {
		idx.Videos = make(map[string]*CachedVideo)
	}
	dm.index = &idx
}

func (dm *DownloadManager) saveIndex() {
	data, err := json.MarshalIndent(dm.index, "", "  ")
	if err != nil {
		log.Printf("[Cache] Failed to marshal index: %v", err)
		return
	}
	os.WriteFile(dm.GetCacheIndexFilePath(), data, 0644)
}

func urlHash(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return fmt.Sprintf("%x", h)
}

func isHLSURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, ".m3u8") || strings.Contains(lower, "mpegurl")
}

func (dm *DownloadManager) GetCachedFile(rawURL string) string {
	dm.indexMu.RLock()
	defer dm.indexMu.RUnlock()

	id := urlHash(rawURL)
	cached, ok := dm.index.Videos[id]
	if !ok {
		return ""
	}

	if _, err := os.Stat(cached.FilePath); os.IsNotExist(err) {
		return ""
	}

	return cached.FilePath
}

func (dm *DownloadManager) ListCachedFiles() []*CachedVideo {
	dm.indexMu.RLock()
	defer dm.indexMu.RUnlock()

	result := make([]*CachedVideo, 0, len(dm.index.Videos))
	for _, v := range dm.index.Videos {
		result = append(result, v)
	}
	return result
}

func (dm *DownloadManager) DeleteCachedFile(rawURL string) bool {
	dm.indexMu.Lock()
	defer dm.indexMu.Unlock()

	id := urlHash(rawURL)
	cached, ok := dm.index.Videos[id]
	if !ok {
		return false
	}

	os.RemoveAll(cached.FilePath)
	delete(dm.index.Videos, id)
	dm.saveIndex()
	return true
}

func (dm *DownloadManager) GetDownloadProgress(id string) *DownloadProgress {
	dm.downloadsMu.RLock()
	defer dm.downloadsMu.RUnlock()
	return dm.downloads[id]
}

func (dm *DownloadManager) DownloadVideo(rawURL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) string {
	id := urlHash(rawURL)

	dm.downloadsMu.Lock()
	if dm.downloads[id] != nil && dm.downloads[id].Status == "downloading" {
		dm.downloadsMu.Unlock()
		return id
	}
	dm.downloads[id] = &DownloadProgress{ID: id, Progress: 0, Status: "downloading"}
	dm.downloadsMu.Unlock()

	go func() {
		if isHLSURL(rawURL) {
			dm.downloadHLS(id, rawURL, headers, videoName, emitProgress)
		} else {
			dm.downloadMP4(id, rawURL, headers, videoName, emitProgress)
		}
	}()

	return id
}

func (dm *DownloadManager) downloadMP4(id string, rawURL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) {
	os.MkdirAll(dm.cacheDir, 0755)

	fileName := fmt.Sprintf("%s_%s.mp4", sanitizeFilename(videoName), id[:8])
	filePath := filepath.Join(dm.cacheDir, fileName)

	dm.updateProgress(id, 0, "downloading", "")
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}

	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		dm.updateProgress(id, 0, "failed", err.Error())
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
		return
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUA2)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		dm.updateProgress(id, 0, "failed", err.Error())
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dm.updateProgress(id, 0, "failed", fmt.Sprintf("HTTP %d", resp.StatusCode))
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
		return
	}

	out, err := os.Create(filePath)
	if err != nil {
		dm.updateProgress(id, 0, "failed", err.Error())
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
		return
	}
	defer out.Close()

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)
	lastProgressEmit := int64(0)

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				dm.updateProgress(id, 0, "failed", writeErr.Error())
				if emitProgress != nil {
					emitProgress(id, *dm.downloads[id])
				}
				os.Remove(filePath)
				return
			}
			downloaded += int64(n)

			if totalSize > 0 {
				progress := float64(downloaded) / float64(totalSize) * 100
				if downloaded-lastProgressEmit > 1024*1024 {
					dm.updateProgress(id, progress, "downloading", "")
					if emitProgress != nil {
						emitProgress(id, *dm.downloads[id])
					}
					lastProgressEmit = downloaded
				}
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			dm.updateProgress(id, 0, "failed", readErr.Error())
			if emitProgress != nil {
				emitProgress(id, *dm.downloads[id])
			}
			os.Remove(filePath)
			return
		}
	}

	out.Close()

	dm.indexMu.Lock()
	dm.index.Videos[id] = &CachedVideo{
		ID:        id,
		URL:       rawURL,
		VideoName: videoName,
		FilePath:  filePath,
		IsHLS:     false,
		Size:      downloaded,
	}
	dm.saveIndex()
	dm.indexMu.Unlock()

	dm.updateProgress(id, 100, "completed", "")
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}
	log.Printf("[Cache] Downloaded MP4: %s (%d bytes)", fileName, downloaded)
}

func (dm *DownloadManager) downloadHLS(id string, m3u8URL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) {
	os.MkdirAll(dm.cacheDir, 0755)

	hlsDir := filepath.Join(dm.cacheDir, fmt.Sprintf("hls_%s", id[:8]))
	os.MkdirAll(hlsDir, 0755)

	dm.updateProgress(id, 0, "downloading", "")
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}

	// Download m3u8 playlist
	m3u8Content, err := dm.downloadURL(m3u8URL, headers)
	if err != nil {
		dm.updateProgress(id, 0, "failed", err.Error())
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
		return
	}

	// Parse m3u8 to get segment URLs
	segments := dm.parseM3U8(string(m3u8Content), m3u8URL)
	if len(segments) == 0 {
		dm.updateProgress(id, 0, "failed", "no segments found")
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
		return
	}

	log.Printf("[Cache] HLS: Found %d segments for %s", len(segments), videoName)

	// Download all segments
	var totalSize int64
	for i, segURL := range segments {
		segFile := filepath.Join(hlsDir, fmt.Sprintf("seg_%05d.ts", i))
		segData, err := dm.downloadURL(segURL, headers)
		if err != nil {
			log.Printf("[Cache] HLS segment %d failed: %v", i, err)
			dm.updateProgress(id, float64(i+1)/float64(len(segments))*100, "downloading", "")
			if emitProgress != nil {
				emitProgress(id, *dm.downloads[id])
			}
			continue
		}
		os.WriteFile(segFile, segData, 0644)
		totalSize += int64(len(segData))

		progress := float64(i+1) / float64(len(segments)) * 100
		dm.updateProgress(id, progress, "downloading", "")
		if emitProgress != nil {
			emitProgress(id, *dm.downloads[id])
		}
	}

	// Rewrite m3u8 to local paths
	localM3U8 := dm.rewriteM3U8ToLocal(string(m3u8Content), hlsDir)
	localM3U8Path := filepath.Join(hlsDir, "playlist.m3u8")
	os.WriteFile(localM3U8Path, []byte(localM3U8), 0644)

	dm.indexMu.Lock()
	dm.index.Videos[id] = &CachedVideo{
		ID:        id,
		URL:       m3u8URL,
		VideoName: videoName,
		FilePath:  localM3U8Path,
		IsHLS:     true,
		Size:      totalSize,
	}
	dm.saveIndex()
	dm.indexMu.Unlock()

	dm.updateProgress(id, 100, "completed", "")
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}
	log.Printf("[Cache] Downloaded HLS: %s (%d segments, %d bytes)", hlsDir, len(segments), totalSize)
}

func (dm *DownloadManager) downloadURL(rawURL string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUA2)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func (dm *DownloadManager) parseM3U8(content string, baseM3U8URL string) []string {
	baseURL := baseM3U8URL
	if idx := strings.LastIndex(baseM3U8URL, "/"); idx >= 0 {
		baseURL = baseM3U8URL[:idx+1]
	}

	lines := strings.Split(content, "\n")
	var segments []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Resolve relative URLs
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			segments = append(segments, trimmed)
		} else if strings.HasPrefix(trimmed, "/") {
			parsedBase, err := url.Parse(baseM3U8URL)
			if err == nil {
				segments = append(segments, fmt.Sprintf("%s://%s%s", parsedBase.Scheme, parsedBase.Host, trimmed))
			}
		} else {
			segments = append(segments, baseURL+trimmed)
		}
	}

	return segments
}

func (dm *DownloadManager) rewriteM3U8ToLocal(content string, hlsDir string) string {
	lines := strings.Split(content, "\n")
	var result []string
	segIndex := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			result = append(result, line)
			continue
		}

		// Replace segment URL with local file
		segFile := filepath.Join(hlsDir, fmt.Sprintf("seg_%05d.ts", segIndex))
		// Use forward slash for m3u8 compatibility
		segFile = strings.ReplaceAll(segFile, "\\", "/")
		result = append(result, segFile)
		segIndex++
	}

	return strings.Join(result, "\n")
}

func (dm *DownloadManager) updateProgress(id string, progress float64, status string, errMsg string) {
	dm.downloadsMu.Lock()
	defer dm.downloadsMu.Unlock()
	dm.downloads[id] = &DownloadProgress{
		ID:       id,
		Progress: progress,
		Status:   status,
		Error:    errMsg,
	}
}

func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	result := replacer.Replace(name)
	if len(result) > 100 {
		result = result[:100]
	}
	return result
}
