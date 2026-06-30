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
	"time"
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
		return filepath.Join(".", "PCBoxCache")
	}
	return filepath.Join(filepath.Dir(exe), "PCBoxCache")
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

func resolveProxyURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if parsed.Host == "" || parsed.Path != "/proxy" {
		return rawURL
	}
	if original := parsed.Query().Get("u"); original != "" {
		origPreview := original
		if len(origPreview) > 80 {
			origPreview = origPreview[:80]
		}
		log.Printf("[Cache] Resolved proxy URL to original: %s", origPreview)
		return original
	}
	return rawURL
}

func (dm *DownloadManager) RetryDownload(id string, emitProgress func(string, DownloadProgress)) bool {
	log.Printf("[Cache] RetryDownload called with id: %s", id)
	var record DownloadRecord
	if dm.cacheDB.db.Where("url_hash = ?", id).First(&record).Error != nil {
		log.Printf("[Cache] RetryDownload: record not found for id: %s", id)
		return false
	}
	log.Printf("[Cache] RetryDownload: found record status=%s url=%s videoName=%s filePath=%s", record.Status, record.URL, record.VideoName, record.FilePath)
	if record.Status != "failed" {
		log.Printf("[Cache] RetryDownload: record status is '%s', not 'failed', skipping", record.Status)
		return false
	}

	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{"status": "pending", "error": "", "progress": 0})

	dm.downloadsMu.Lock()
	dm.downloads[id] = &DownloadProgress{ID: id, Progress: 0, Status: "downloading"}
	dm.downloadsMu.Unlock()

	downloadURL := resolveProxyURL(record.URL)
	log.Printf("[Cache] RetryDownload: launching executeDownload goroutine for %s", record.VideoName)
	go dm.executeDownload(id, downloadURL, record.Headers, record.VideoName, emitProgress)
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
		downloadURL := resolveProxyURL(r.URL)
		go dm.executeDownload(r.URLHash, downloadURL, r.Headers, r.VideoName, emitProgress)
	}
}

func (dm *DownloadManager) DownloadVideo(rawURL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) string {
	return dm.DownloadVideoWithMeta(rawURL, headers, videoName, "", "", -1, "", "", emitProgress)
}

func (dm *DownloadManager) DownloadVideoWithMeta(rawURL string, headers map[string]string, videoName string, sourceKey string, playFlag string, episodeIndex int, vodId string, vodPic string, emitProgress func(string, DownloadProgress)) string {
	id := urlHash(rawURL)

	dm.downloadsMu.Lock()
	if dm.downloads[id] != nil && dm.downloads[id].Status == "downloading" {
		dm.downloadsMu.Unlock()
		return id
	}
	dm.downloadsMu.Unlock()

	headersJSON, _ := json.Marshal(headers)
	record := &DownloadRecord{
		URLHash:      id,
		URL:          rawURL,
		Headers:      string(headersJSON),
		VideoName:    videoName,
		SourceKey:    sourceKey,
		PlayFlag:     playFlag,
		EpisodeIndex: episodeIndex,
		VodId:        vodId,
		VodPic:       vodPic,
		Status:       "pending",
		Progress:     0,
	}
	dm.cacheDB.db.Create(record)

	dm.downloadsMu.Lock()
	dm.downloads[id] = &DownloadProgress{ID: id, Progress: 0, Status: "downloading"}
	dm.downloadsMu.Unlock()

	go dm.executeDownload(id, rawURL, string(headersJSON), videoName, emitProgress)

	return id
}

func (dm *DownloadManager) executeDownload(id string, rawURL string, headersJSON string, videoName string, emitProgress func(string, DownloadProgress)) {
	urlPreview := rawURL
	if len(urlPreview) > 80 {
		urlPreview = urlPreview[:80]
	}
	log.Printf("[Cache] executeDownload: START for %s (id=%s, isHLS=%v, url=%s)", videoName, id, isHLSURL(rawURL), urlPreview)

	var headers map[string]string
	if headersJSON != "" {
		if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
			log.Printf("[Cache] executeDownload: failed to parse headers: %v", err)
		}
	}

	dm.cacheDB.db.Model(&DownloadRecord{}).Where("url_hash = ?", id).
		Updates(map[string]interface{}{"status": "downloading", "progress": 0})

	dm.downloadsMu.Lock()
	dm.downloads[id] = &DownloadProgress{ID: id, Progress: 0, Status: "downloading"}
	dm.downloadsMu.Unlock()

	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}

	if isHLSURL(rawURL) {
		log.Printf("[Cache] executeDownload: calling downloadHLS for %s", videoName)
		dm.downloadHLS(id, rawURL, headers, videoName, emitProgress)
	} else {
		log.Printf("[Cache] executeDownload: calling downloadMP4 for %s", videoName)
		dm.downloadMP4(id, rawURL, headers, videoName, emitProgress)
	}
	log.Printf("[Cache] executeDownload: DONE for %s", videoName)
}

func (dm *DownloadManager) downloadMP4(id string, rawURL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) {
	log.Printf("[Cache] downloadMP4: starting for %s", videoName)
	os.MkdirAll(dm.cacheDir, 0755)

	fileName := fmt.Sprintf("%s_%s.mp4", sanitizeFilename(videoName), id[:8])
	filePath := filepath.Join(dm.cacheDir, fileName)
	filePath, _ = filepath.Abs(filePath)

	var existingSize int64
	if fi, err := os.Stat(filePath); err == nil {
		existingSize = fi.Size()
		log.Printf("[Cache] downloadMP4: found partial file %s (%d bytes), will resume", filePath, existingSize)
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

	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		dm.failDownload(id, err.Error(), emitProgress)
		return
	}
	defer resp.Body.Close()

	resumed := false
	if existingSize > 0 && resp.StatusCode == http.StatusPartialContent {
		resumed = true
	} else if existingSize > 0 && resp.StatusCode == http.StatusOK {
		existingSize = 0
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		dm.failDownload(id, fmt.Sprintf("HTTP %d", resp.StatusCode), emitProgress)
		return
	}

	var out *os.File
	if resumed {
		out, err = os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		out, err = os.Create(filePath)
	}
	if err != nil {
		dm.failDownload(id, err.Error(), emitProgress)
		return
	}
	defer out.Close()

	totalSize := resp.ContentLength
	if resumed && totalSize > 0 {
		totalSize += existingSize
	}
	var downloaded int64
	if resumed {
		downloaded = existingSize
	}
	buf := make([]byte, 32*1024)
	lastProgressEmit := downloaded

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				dm.failDownload(id, writeErr.Error(), emitProgress)
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
	log.Printf("[Cache] Downloaded MP4: %s (%d bytes, resumed=%v)", fileName, downloaded, resumed)
}

func (dm *DownloadManager) downloadHLS(id string, m3u8URL string, headers map[string]string, videoName string, emitProgress func(string, DownloadProgress)) {
	os.MkdirAll(dm.cacheDir, 0755)

	hlsDir := filepath.Join(dm.cacheDir, fmt.Sprintf("hls_%s", id[:8]))
	hlsDir, _ = filepath.Abs(hlsDir)
	os.MkdirAll(hlsDir, 0755)

	dm.updateProgress(id, 0, "downloading", "")
	if emitProgress != nil {
		emitProgress(id, *dm.downloads[id])
	}

	m3u8Content, err := dm.downloadURL(m3u8URL, headers)
	if err != nil {
		log.Printf("[Cache] HLS: failed to download m3u8: %v", err)
		dm.failDownload(id, err.Error(), emitProgress)
		return
	}

	log.Printf("[Cache] HLS: m3u8 downloaded (%d bytes), parsing...", len(m3u8Content))

	segments := dm.parseM3U8(string(m3u8Content), m3u8URL)
	if len(segments) == 0 {
		dm.failDownload(id, "no segments found", emitProgress)
		return
	}

	log.Printf("[Cache] HLS: Found %d segments for %s", len(segments), videoName)

	var totalSize int64
	for i, segURL := range segments {
		segFile := filepath.Join(hlsDir, fmt.Sprintf("seg_%05d.ts", i))
		if _, err := os.Stat(segFile); err == nil {
			totalSize += func() int64 {
				fi, _ := os.Stat(segFile)
				if fi != nil {
					return fi.Size()
				}
				return 0
			}()
			continue
		}
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
	if len(rawURL) > 80 {
		log.Printf("[Cache] downloadURL: fetching %s...", rawURL[:80])
	} else {
		log.Printf("[Cache] downloadURL: fetching %s", rawURL)
	}
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
		log.Printf("[Cache] downloadURL: request failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Cache] downloadURL: HTTP %d for %s", resp.StatusCode, rawURL[:min(80, len(rawURL))])
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	log.Printf("[Cache] downloadURL: success, reading body...")
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

		segFile := fmt.Sprintf("seg_%05d.ts", segIndex)
		result = append(result, segFile)
		segIndex++
	}

	return strings.Join(result, "\n")
}

type PlayHistoryEntry struct {
	SourceKey    string `json:"sourceKey,omitempty"`
	VodId        string `json:"vodId,omitempty"`
	VodName      string `json:"vodName"`
	VodPic       string `json:"vodPic,omitempty"`
	PlayFlag     string `json:"playFlag"`
	EpisodeFlag  string `json:"episodeFlag"`
	EpisodeUrl   string `json:"episodeUrl"`
	EpisodeIndex int    `json:"episodeIndex"`
	ReverseSort  bool   `json:"reverseSort"`
	Progress     int    `json:"progress"`
	Duration     int    `json:"duration"`
	UpdatedAt    int64  `json:"updatedAt"`
}

func (dm *DownloadManager) historyFilePath() string {
	return filepath.Join(dm.cacheDir, "play_history.json")
}

func (dm *DownloadManager) loadAllHistory() []*PlayHistoryEntry {
	path := dm.historyFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []*PlayHistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

func (dm *DownloadManager) saveAllHistory(entries []*PlayHistoryEntry) {
	path := dm.historyFilePath()
	data, _ := json.MarshalIndent(entries, "", "  ")
	os.WriteFile(path, data, 0644)
}

func (dm *DownloadManager) SavePlayHistory(entry PlayHistoryEntry) {
	entries := dm.loadAllHistory()
	// Replace existing entry for same episodeUrl, or append
	found := false
	for i, e := range entries {
		if e.EpisodeUrl == entry.EpisodeUrl {
			entries[i] = &entry
			found = true
			break
		}
	}
	if !found {
		entries = append(entries, &entry)
	}
	// Keep only last 200 entries
	if len(entries) > 200 {
		entries = entries[len(entries)-200:]
	}
	dm.saveAllHistory(entries)
}

func (dm *DownloadManager) GetPlayHistory() []*PlayHistoryEntry {
	entries := dm.loadAllHistory()
	if entries == nil {
		return []*PlayHistoryEntry{}
	}
	return entries
}

func (dm *DownloadManager) FindDownloadRecordByFilePath(filePath string) *DownloadRecord {
	var record DownloadRecord
	result := dm.cacheDB.db.Where("file_path = ?", filePath).First(&record)
	if result.Error != nil {
		return nil
	}
	return &record
}

func (dm *DownloadManager) FindNextCachedEpisode(sourceKey string, playFlag string, episodeIndex int) *DownloadRecord {
	if sourceKey == "" || playFlag == "" {
		return nil
	}
	var record DownloadRecord
	result := dm.cacheDB.db.Where("source_key = ? AND play_flag = ? AND episode_index = ? AND status = ?",
		sourceKey, playFlag, episodeIndex, "completed").First(&record)
	if result.Error != nil {
		return nil
	}
	if _, err := os.Stat(record.FilePath); os.IsNotExist(err) {
		return nil
	}
	return &record
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

type CacheProgressEntry struct {
	FilePath  string    `json:"filePath"`
	Progress  int       `json:"progress"`
	Duration  int       `json:"duration"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (dm *DownloadManager) progressFilePath() string {
	return filepath.Join(dm.cacheDir, "cache_progress.json")
}

func (dm *DownloadManager) loadAllProgress() map[string]*CacheProgressEntry {
	path := dm.progressFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]*CacheProgressEntry)
	}
	var m map[string]*CacheProgressEntry
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]*CacheProgressEntry)
	}
	return m
}

func (dm *DownloadManager) saveAllProgress(m map[string]*CacheProgressEntry) {
	path := dm.progressFilePath()
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(path, data, 0644)
}

func (dm *DownloadManager) SaveCacheProgress(filePath string, progress int, duration int) {
	m := dm.loadAllProgress()
	m[filePath] = &CacheProgressEntry{
		FilePath:  filePath,
		Progress:  progress,
		Duration:  duration,
		UpdatedAt: time.Now(),
	}
	dm.saveAllProgress(m)
}

func (dm *DownloadManager) GetCacheProgress(filePath string) map[string]interface{} {
	m := dm.loadAllProgress()
	entry, ok := m[filePath]
	if !ok {
		return map[string]interface{}{"progress": 0, "duration": 0}
	}
	return map[string]interface{}{
		"progress": entry.Progress,
		"duration": entry.Duration,
	}
}
