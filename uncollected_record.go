package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// UncollectedFileRecord 取消收藏文件记录
type UncollectedFileRecord struct {
	Filename        string `json:"filename"`
	UncollectedAt   string `json:"uncollected_at"` // RFC3339 格式时间戳
}

// UncollectedFilesManager 取消收藏文件管理器
type UncollectedFilesManager struct {
	filePath string
	records  map[string]UncollectedFileRecord // key: filename
	mu       sync.RWMutex
}

// NewUncollectedFilesManager 创建取消收藏文件管理器
func NewUncollectedFilesManager(audioDir string) *UncollectedFilesManager {
	// 使用 audio_dir 所在目录存储 uncollected_files.json
	storageDir := filepath.Dir(audioDir)
	filePath := filepath.Join(storageDir, "uncollected_files.json")

	manager := &UncollectedFilesManager{
		filePath: filePath,
		records:  make(map[string]UncollectedFileRecord),
	}

	// 加载现有记录
	manager.load()

	return manager
}

// load 加载取消收藏文件记录
func (m *UncollectedFilesManager) load() error {
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

	var records []UncollectedFileRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	// 转换为 map
	m.records = make(map[string]UncollectedFileRecord)
	for _, record := range records {
		m.records[record.Filename] = record
	}

	return nil
}

// save 保存取消收藏文件记录
func (m *UncollectedFilesManager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 转换为数组
	records := make([]UncollectedFileRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

// Add 添加取消收藏记录
func (m *UncollectedFilesManager) Add(filename string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records[filename] = UncollectedFileRecord{
		Filename:      filename,
		UncollectedAt: time.Now().Format(time.RFC3339),
	}

	// 异步保存
	go m.save()
}

// IsUncollected 检查文件是否已取消收藏
func (m *UncollectedFilesManager) IsUncollected(filename string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.records[filename]
	return exists
}

// GetAll 获取所有取消收藏记录
func (m *UncollectedFilesManager) GetAll() []UncollectedFileRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]UncollectedFileRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}
	return records
}

// Cleanup 清理超过指定天数的记录
func (m *UncollectedFilesManager) Cleanup(days int) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	count := 0

	for filename, record := range m.records {
		uncollectedAt, err := time.Parse(time.RFC3339, record.UncollectedAt)
		if err != nil {
			continue
		}

		if uncollectedAt.Before(cutoff) {
			delete(m.records, filename)
			count++
		}
	}

	if count > 0 {
		go m.save()
	}

	return count
}

// Remove 删除取消收藏记录
// 返回 true 表示找到并删除，false 表示未找到
func (m *UncollectedFilesManager) Remove(filename string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.records[filename]
	if !exists {
		return false
	}

	delete(m.records, filename)

	// 异步保存
	go m.save()

	return true
}