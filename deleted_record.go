package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeletedFileRecord 已删除文件记录
type DeletedFileRecord struct {
	Filename string `json:"filename"`
	DeletedAt string `json:"deleted_at"`
}

// DeletedFilesManager 已删除文件管理器
type DeletedFilesManager struct {
	filePath string
	records  map[string]DeletedFileRecord // key: filename
	mu       sync.RWMutex
}

// NewDeletedFilesManager 创建已删除文件管理器
func NewDeletedFilesManager(audioDir string) *DeletedFilesManager {
	// 使用 audio_dir 所在目录存储 deleted_files.json
	storageDir := filepath.Dir(audioDir)
	filePath := filepath.Join(storageDir, "deleted_files.json")

	manager := &DeletedFilesManager{
		filePath: filePath,
		records:  make(map[string]DeletedFileRecord),
	}

	// 加载现有记录
	manager.load()

	return manager
}

// load 加载已删除文件记录
func (m *DeletedFilesManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，初始化空记录
			return nil
		}
		return err
	}

	var records []DeletedFileRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	// 转换为 map
	m.records = make(map[string]DeletedFileRecord)
	for _, record := range records {
		m.records[record.Filename] = record
	}

	return nil
}

// save 保存已删除文件记录
func (m *DeletedFilesManager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 转换为数组
	records := make([]DeletedFileRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

// Add 添加删除记录
func (m *DeletedFilesManager) Add(filename string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records[filename] = DeletedFileRecord{
		Filename: filename,
		DeletedAt: time.Now().Format(time.RFC3339),
	}

	// 异步保存
	go m.save()
}

// IsDeleted 检查文件是否已删除
func (m *DeletedFilesManager) IsDeleted(filename string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.records[filename]
	return exists
}

// GetAll 获取所有删除记录
func (m *DeletedFilesManager) GetAll() []DeletedFileRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]DeletedFileRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}
	return records
}

// Cleanup 清理超过指定天数的记录
func (m *DeletedFilesManager) Cleanup(days int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	count := 0

	for filename, record := range m.records {
		deletedAt, err := time.Parse(time.RFC3339, record.DeletedAt)
		if err != nil {
			continue
		}

		if deletedAt.Before(cutoff) {
			delete(m.records, filename)
			count++
		}
	}

	if count > 0 {
		go m.save()
	}

	return count
}
