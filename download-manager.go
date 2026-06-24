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

type DownloadProgress struct {
	ID       string  `json:"id"`
	Progress float64 `json:"progress"`
	Status   string  `json:"status"` // "downloading", "completed", "failed"
	Error    string  `json:"error,omitempty"`
}

type CachedVideo struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	VideoName string `json:"videoName"`
	FilePath  string `json:"filePath"`
	IsHLS     bool   `json:"isHLS"`
	Size      int64  `json:"size"`
}

type CacheIndex struct {
	Videos map[string]*CachedVideo `json:"videos"`
}

type DownloadManager struct {
	cacheDir    string
	cacheDB     *CacheDB
	downloads   map[string]*DownloadProgress
	downloadsMu sync.RWMutex
}

func NewDownloadManager() *DownloadManager {
	dm := &DownloadManager{
		downloads: make(map[string]*DownloadProgress),
	}
	dm.cacheDir = dm.defaultCacheDir()
	dm.cacheDB = NewCacheDB(dm.cacheDir)
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
	dm.cacheDir = dir
	os.MkdirAll(dir, 0755)
	dm.cacheDB = NewCacheDB(dir)
}

func (dm *DownloadManager) GetCacheDir() string {
	return dm.cacheDir
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
	id := urlHash(rawURL)
	var record DownloadRecord
	result := dm.cacheDB.db.Where("url_hash = ? AND status = ?", id, "completed").First(&record)
	if result.Error != nil {
		return ""
	}
	if _, err := os.Stat(record.FilePath); os.IsNotExist(err) {
		return ""
	}
	return record.FilePath
}

func (dm *DownloadManager) ListCachedFiles() []*CachedVideo {
	var records []DownloadRecord
	dm.cacheDB.db.Where("status = ?", "completed").Find(&records)
	result := make([]*CachedVideo, 0, len(records))
	for _, r := range records {
		result = append(result, &CachedVideo{
			ID:        r.URLHash,
			URL:       r.URL,
			VideoName: r.VideoName,
			FilePath:  r.FilePath,
			IsHLS:     r.IsHLS,
			Size:      r.Size,
		})
	}
	return result
}

func (dm *DownloadManager) ListCachedFilesPaged(page, pageSize int, keyword string, status string) ([]DownloadRecord, int64) {
	query := dm.cacheDB.db
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		query = query.Where("video_name LIKE ?", "%"+keyword+"%")
	}
	var total int64
	query.Model(&DownloadRecord{}).Count(&total)
	var records []DownloadRecord
	query.Order("updated_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)
	return records, total
}

func (dm *DownloadManager) DeleteCachedFile(rawURL string) bool {
	id := urlHash(rawURL)
	var record DownloadRecord
	result := dm.cacheDB.db.Where("url_hash = ?", id).First(&record)
	if result.Error != nil {
		return false
	}
	if record.FilePath != "" {
		os.RemoveAll(record.FilePath)
		hlsDir := filepath.Join(dm.cacheDir, fmt.Sprintf("hls_%s", id[:8]))
		os.RemoveAll(hlsDir)
	}
	dm.cacheDB.db.Delete(&record)
	return true
}

func (dm *DownloadManager) DeleteCacheByID(id uint) bool {
	var record DownloadRecord
	result := dm.cacheDB.db.First(&record, id)
	if result.Error != nil {
		return false
	}
	if record.FilePath != "" {
		os.RemoveAll(record.FilePath)
	}
	if record.IsHLS {
		hlsDir := filepath.Join(dm.cacheDir, fmt.Sprintf("hls_%s", record.URLHash[:8]))
		os.RemoveAll(hlsDir)
	}
	dm.cacheDB.db.Delete(&record)
	return true
}

func (dm *DownloadManager) DeleteCacheBatch(ids []uint) int {
	var records []DownloadRecord
	dm.cacheDB.db.Where("id IN ?", ids).Find(&records)
	deleted := 0
	for _, r := range records {
		if r.FilePath != "" {
			os.RemoveAll(r.FilePath)
		}
		if r.IsHLS {
			hlsDir := filepath.Join(dm.cacheDir, fmt.Sprintf("hls_%s", r.URLHash[:8]))
			os.RemoveAll(hlsDir)
		}
		dm.cacheDB.db.Delete(&r)
		deleted++
	}
	return deleted
}

func (dm *DownloadManager) GetCacheStats() map[string]interface{} {
	var total int64
	dm.cacheDB.db.Model(&DownloadRecord{}).Where("status = ?", "completed").Count(&total)
	var totalSize int64
	dm.cacheDB.db.Model(&DownloadRecord{}).Where("status = ?", "completed").Select("COALESCE(SUM(size), 0)").Scan(&totalSize)
	var pending int64
	dm.cacheDB.db.Model(&DownloadRecord{}).Where("status IN ?", []string{"pending", "downloading"}).Count(&pending)
	return map[string]interface{}{
		"total":   total,
		"totalSize": totalSize,
		"pending": pending,
	}
}

func (dm *DownloadManager) GetDownloadProgress(id string) *DownloadProgress {
	dm.downloadsMu.RLock()
	defer dm.downloadsMu.RUnlock()
	return dm.downloads[id]
}

func (dm *DownloadManager) GetDownloadQueue() []DownloadRecord {
	var records []DownloadRecord
	dm.cacheDB.db.Where("status IN ?", []string{"pending", "downloading"}).Order("created_at ASC").Find(&records)
	return records
}

func (dm *DownloadManager) CancelDownload(id string) bool {
	dm.downloadsMu.Lock()
	if dp, ok := dm.downloads[id]; ok {
		dp.Status = "failed"
		dp.Error = "cancelled"
		dm.downloadsMu.Unlock()
	} else {
		dm.downloadsMu.Unlock()
	}
	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{"status": "failed", "error": "cancelled", "progress": 0})
	var record DownloadRecord
	if dm.cacheDB.db.Where("url_hash = ?", id).First(&record).Error == nil {
		if record.FilePath != "" {
			os.RemoveAll(record.FilePath)
		}
	}
	return true
}

func (dm *DownloadManager) ResumePendingDownloads(emitProgress func(string, DownloadProgress)) {
	var records []DownloadRecord
	dm.cacheDB.db.Where("status IN ?", []string{"pending", "downloading"}).Find(&records)
	if len(records) == 0 {
		return
	}
	log.Printf("[Cache] Resuming %d pending downloads", len(records))
	for i := range records {
		r := &records[i]
		r.Status = "pending"
		r.Progress = 0
		dm.cacheDB.db.Save(r)
		go dm.executeDownload(r.URL, r.Headers, r.VideoName, emitProgress)
	}
}

func (dm *DownloadManager) DownloadVideo(rawURL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) string {
	id := urlHash(rawURL)

	dm.downloadsMu.Lock()
	if dm.downloads[id] != nil && dm.downloads[id].Status == "downloading" {
		dm.downloadsMu.Unlock()
		return id
	}
	dm.downloadsMu.Unlock()

	headersJSON, _ := json.Marshal(headers)
	record := &DownloadRecord{
		URLHash:   id,
		URL:       rawURL,
		Headers:   string(headersJSON),
		VideoName: videoName,
		Status:    "pending",
		Progress:  0,
	}
	dm.cacheDB.db.Create(record)

	dm.downloadsMu.Lock()
	dm.downloads[id] = &DownloadProgress{ID: id, Progress: 0, Status: "downloading"}
	dm.downloadsMu.Unlock()

	go dm.executeDownload(rawURL, string(headersJSON), videoName, emitProgress)

	return id
}

func (dm *DownloadManager) executeDownload(rawURL string, headersJSON string, videoName string, emitProgress func(string, DownloadProgress)) {
	id := urlHash(rawURL)

	var headers map[string]string
	json.Unmarshal([]byte(headersJSON), &headers)

	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{"status": "downloading", "progress": 0})

	dm.downloadsMu.Lock()
	dm.downloads[id] = &DownloadProgress{ID: id, Progress: 0, Status: "downloading"}
	dm.downloadsMu.Unlock()

	if isHLSURL(rawURL) {
		dm.downloadHLS(id, rawURL, headers, videoName, emitProgress)
	} else {
		dm.downloadMP4(id, rawURL, headers, videoName, emitProgress)
	}
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
		dm.failDownload(id, err.Error(), emitProgress)
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
		dm.failDownload(id, err.Error(), emitProgress)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dm.failDownload(id, fmt.Sprintf("HTTP %d", resp.StatusCode), emitProgress)
		return
	}

	out, err := os.Create(filePath)
	if err != nil {
		dm.failDownload(id, err.Error(), emitProgress)
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
				dm.failDownload(id, writeErr.Error(), emitProgress)
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
			dm.failDownload(id, readErr.Error(), emitProgress)
			os.Remove(filePath)
			return
		}
	}

	out.Close()

	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{
			"file_path": filePath,
			"is_hls":    false,
			"size":      downloaded,
			"status":    "completed",
			"progress":  100,
		})

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

	m3u8Content, err := dm.downloadURL(m3u8URL, headers)
	if err != nil {
		dm.failDownload(id, err.Error(), emitProgress)
		return
	}

	segments := dm.parseM3U8(string(m3u8Content), m3u8URL)
	if len(segments) == 0 {
		dm.failDownload(id, "no segments found", emitProgress)
		return
	}

	log.Printf("[Cache] HLS: Found %d segments for %s", len(segments), videoName)

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

	localM3U8 := dm.rewriteM3U8ToLocal(string(m3u8Content), hlsDir)
	localM3U8Path := filepath.Join(hlsDir, "playlist.m3u8")
	os.WriteFile(localM3U8Path, []byte(localM3U8), 0644)

	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{
			"file_path": localM3U8Path,
			"is_hls":    true,
			"size":      totalSize,
			"status":    "completed",
			"progress":  100,
		})

	dm.updateProgress(id, 100, "completed", "")
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}
	log.Printf("[Cache] Downloaded HLS: %s (%d segments, %d bytes)", hlsDir, len(segments), totalSize)
}

func (dm *DownloadManager) failDownload(id string, errMsg string, emitProgress func(string, DownloadProgress)) {
	dm.updateProgress(id, 0, "failed", errMsg)
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}
	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{"status": "failed", "error": errMsg, "progress": 0})
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

		segFile := filepath.Join(hlsDir, fmt.Sprintf("seg_%05d.ts", segIndex))
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
