package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DownloadRecord struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	URLHash      string    `gorm:"uniqueIndex;size:64" json:"urlHash"`
	URL          string    `gorm:"type:text" json:"url"`
	Headers      string    `gorm:"type:text" json:"headers"`
	VideoName    string    `gorm:"type:text" json:"videoName"`
	FilePath     string    `gorm:"type:text" json:"filePath"`
	IsHLS        bool      `json:"isHLS"`
	Size         int64     `json:"size"`
	Status       string    `gorm:"size:20;index" json:"status"`
	Progress     float64   `json:"progress"`
	Error        string    `gorm:"type:text" json:"error,omitempty"`
	SourceKey    string    `gorm:"size:64;index" json:"sourceKey,omitempty"`
	PlayFlag     string    `gorm:"size:32" json:"playFlag,omitempty"`
	EpisodeIndex int       `gorm:"index" json:"episodeIndex"`
	VodId        string    `gorm:"size:64" json:"vodId,omitempty"`
	VodPic       string    `gorm:"type:text" json:"vodPic,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CacheDB struct {
	db *gorm.DB
}

func NewCacheDB(cacheDir string) *CacheDB {
	os.MkdirAll(cacheDir, 0755)
	dbPath := filepath.Join(cacheDir, "cache.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("[CacheDB] Failed to open database: %v", err)
	}
	db.AutoMigrate(&DownloadRecord{})
	cdb := &CacheDB{db: db}
	cdb.migrateFromJSON(cacheDir)
	return cdb
}

func (cdb *CacheDB) migrateFromJSON(cacheDir string) {
	var count int64
	cdb.db.Model(&DownloadRecord{}).Count(&count)
	if count > 0 {
		return
	}
	jsonPath := filepath.Join(cacheDir, "cache-index.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return
	}
	var idx CacheIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		log.Printf("[CacheDB] Failed to parse cache-index.json: %v", err)
		return
	}
	if idx.Videos == nil || len(idx.Videos) == 0 {
		return
	}
	log.Printf("[CacheDB] Migrating %d entries from cache-index.json", len(idx.Videos))
	for _, v := range idx.Videos {
		record := &DownloadRecord{
			URLHash:   v.ID,
			URL:       v.URL,
			VideoName: v.VideoName,
			FilePath:  v.FilePath,
			IsHLS:     v.IsHLS,
			Size:      v.Size,
			Status:    "completed",
			Progress:  100,
		}
		cdb.db.Create(record)
	}
	backupPath := jsonPath + ".bak"
	os.Rename(jsonPath, backupPath)
	log.Printf("[CacheDB] Migration complete, backup saved to %s", backupPath)
}

func (cdb *CacheDB) GetDB() *gorm.DB {
	return cdb.db
}
