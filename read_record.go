package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ReadFileRecord 已读文件记录
type ReadFileRecord struct {
	Filename string `json:"filename"`
	ReadAt   string `json:"read_at"` // RFC3339 格式时间戳
}

// ReadFilesManager 已读文件管理器
type ReadFilesManager struct {
	filePath string
	records  map[string]ReadFileRecord // key: filename
	mu       sync.RWMutex
}

// NewReadFilesManager 创建已读文件管理器
func NewReadFilesManager(audioDir string) *ReadFilesManager {
	// 使用 audio_dir 所在目录存储 read_files.json
	storageDir := filepath.Dir(audioDir)
	filePath := filepath.Join(storageDir, "read_files.json")

	manager := &ReadFilesManager{
		filePath: filePath,
		records:  make(map[string]ReadFileRecord),
	}

	// 加载现有记录
	manager.load()

	return manager
}

// load 加载已读文件记录
func (m *ReadFilesManager) load() error {
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

	var records []ReadFileRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	// 转换为 map
	m.records = make(map[string]ReadFileRecord)
	for _, record := range records {
		m.records[record.Filename] = record
	}

	return nil
}

// save 保存已读文件记录
func (m *ReadFilesManager) save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 转换为数组
	records := make([]ReadFileRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filePath, data, 0644)
}

// Mark 标记文件为已读
func (m *ReadFilesManager) Mark(filename string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.records[filename] = ReadFileRecord{
		Filename: filename,
		ReadAt:   time.Now().Format(time.RFC3339),
	}

	// 异步保存
	go m.save()
}

// IsRead 检查文件是否已读
func (m *ReadFilesManager) IsRead(filename string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.records[filename]
	return exists
}

// GetAll 获取所有已读记录
func (m *ReadFilesManager) GetAll() []ReadFileRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]ReadFileRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}
	return records
}

// GetByFilename 按文件名获取已读记录
func (m *ReadFilesManager) GetByFilename(filename string) (*ReadFileRecord, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[filename]
	if !exists {
		return nil, false
	}
	return &record, true
}