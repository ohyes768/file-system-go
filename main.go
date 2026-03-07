package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/yaml.v3"
)

// 配置结构体
type Config struct {
	Server struct {
		Port         string `yaml:"port"`
		ReadTimeout  int    `yaml:"read_timeout"`
		WriteTimeout int    `yaml:"write_timeout"`
	} `yaml:"server"`
	Storage struct {
		AudioDir    string `yaml:"audio_dir"`
		MaxUploadMB int    `yaml:"max_upload_mb"`
	} `yaml:"storage"`
	Logging struct {
		Level  string `yaml:"level"`
		LogDir string `yaml:"log_dir"`
	} `yaml:"logging"`
}

// 响应结构体
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type UploadResponse struct {
	Success  bool           `json:"success"`
	Filename string         `json:"filename,omitempty"`
	URL      string         `json:"url,omitempty"`
	Size     int64          `json:"size,omitempty"`
	Metadata *VideoMetadata `json:"metadata,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type HealthResponse struct {
	Service        string `json:"service"`
	Status         string `json:"status"`
	Version        string `json:"version"`
	UploadEndpoint string `json:"upload_endpoint"`
	AudioDir       string `json:"audio_dir"`
}

type CheckFileResponse struct {
	Exists     bool   `json:"exists"`
	Filename   string `json:"filename"`
	Size       int64  `json:"size,omitempty"`
	UploadTime string `json:"upload_time,omitempty"`
}

// 视频元数据
type VideoMetadata struct {
	Filename    string    `json:"filename"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	UploadTime  time.Time `json:"upload_time"`
}

// 元数据响应
type MetadataResponse struct {
	Success  bool           `json:"success"`
	Metadata *VideoMetadata `json:"metadata,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// 删除文件响应
type DeleteFileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// 视频文件信息
type VideoFile struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	URL      string `json:"url,omitempty"`
}

// 查询请求
type QueryRequest struct {
	Filters *QueryFilters `json:"filters,omitempty"`
}

type QueryFilters struct {
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
}

// 查询响应
type QueryResponse struct {
	Success bool        `json:"success"`
	Videos  []VideoFile `json:"videos,omitempty"`
	Error   string      `json:"error,omitempty"`
}

var (
	config               Config
	fileLogger           *log.Logger
	consoleLogger         = log.New(os.Stdout, "", log.LstdFlags)
	deletedFilesManager   *DeletedFilesManager
	readFilesManager      *ReadFilesManager
	uncollectedFilesManager *UncollectedFilesManager
)

// 加载配置文件
func loadConfig() error {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		consoleLogger.Printf("警告: 无法读取配置文件，使用默认配置: %v", err)
		setDefaultConfig()
		return nil
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("配置文件解析失败: %v", err)
	}

	consoleLogger.Println("配置文件加载成功")
	return nil
}

// 设置默认配置
func setDefaultConfig() {
	config.Server.Port = "8000"
	config.Server.ReadTimeout = 300
	config.Server.WriteTimeout = 300
	config.Storage.AudioDir = "./audio_files"
	config.Storage.MaxUploadMB = 100
	config.Logging.Level = "INFO"
	config.Logging.LogDir = "./logs"
}

// 初始化日志
func initLogger() error {
	// 确保日志目录存在
	if err := os.MkdirAll(config.Logging.LogDir, 0755); err != nil {
		return fmt.Errorf("无法创建日志目录: %v", err)
	}

	// 创建日志文件（按日期命名）
	dateStr := time.Now().Format("2006-01-02")
	logFile := filepath.Join(config.Logging.LogDir, fmt.Sprintf("audio-server-%s.log", dateStr))

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("无法创建日志文件: %v", err)
	}

	// 创建多输出写入器（控制台+文件）
	multiWriter := io.MultiWriter(os.Stdout, file)
	fileLogger = log.New(multiWriter, "", log.LstdFlags)

	return nil
}

// 保存元数据文件
func saveMetadata(metadata VideoMetadata) error {
	// 构建元数据文件路径：filename.mp4.meta.json
	metaFilePath := filepath.Join(config.Storage.AudioDir, metadata.Filename+".meta.json")

	// 序列化为 JSON（带缩进，便于阅读）
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("元数据序列化失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(metaFilePath, data, 0644); err != nil {
		return fmt.Errorf("元数据文件写入失败: %v", err)
	}

	fileLogger.Printf("元数据已保存: %s", metaFilePath)
	return nil
}

// 加载元数据文件
func loadMetadata(filename string) (*VideoMetadata, error) {
	// 构建元数据文件路径
	metaFilePath := filepath.Join(config.Storage.AudioDir, filename+".meta.json")

	// 读取文件
	data, err := os.ReadFile(metaFilePath)
	if err != nil {
		return nil, err
	}

	// 反序列化
	var metadata VideoMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("元数据解析失败: %v", err)
	}

	return &metadata, nil
}

// 健康检查接口
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{
		Service:        "Audio File Server (Go)",
		Status:         "running",
		Version:        "1.5.0",
		UploadEndpoint: "/upload",
		AudioDir:       config.Storage.AudioDir,
	}
	json.NewEncoder(w).Encode(response)
}

// 文件上传接口
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 限制上传大小
	r.Body = http.MaxBytesReader(w, r.Body, int64(config.Storage.MaxUploadMB)<<20)

	// 解析表单
	if err := r.ParseMultipartForm(int64(config.Storage.MaxUploadMB) << 20); err != nil {
		fileLogger.Printf("解析表单失败: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "文件过大或解析失败",
		})
		return
	}
	fileLogger.Printf("表单解析成功")

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		fileLogger.Printf("获取文件失败: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "未找到上传文件",
		})
		return
	}
	fileLogger.Printf("获取文件成功: %s", header.Filename)
	defer file.Close()

	// 构建文件保存路径
	filename := header.Filename
	filePath := filepath.Join(config.Storage.AudioDir, filename)

	// 创建目标文件
	dst, err := os.Create(filePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("无法创建文件: %v", err),
		})
		return
	}
	defer dst.Close()

	// 复制文件内容
	size, err := io.Copy(dst, file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("文件保存失败: %v", err),
		})
		return
	}

	fileLogger.Printf("文件上传成功: %s (%d bytes) 来自 %s", filename, size, r.RemoteAddr)

	// 获取元数据参数
	title := r.FormValue("title")
	author := r.FormValue("author")
	description := r.FormValue("description")

	// 创建元数据
	metadata := VideoMetadata{
		Filename:    filename,
		Title:       title,
		Author:      author,
		Description: description,
		UploadTime:  time.Now(),
	}

	// 保存元数据文件
	if err := saveMetadata(metadata); err != nil {
		fileLogger.Printf("警告: 元数据保存失败: %v", err)
		// 元数据保存失败不影响文件上传成功
	}

	// 构建访问 URL
	fileURL := fmt.Sprintf("/audio/%s", filename)

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		Filename: filename,
		URL:      fileURL,
		Size:     size,
		Metadata: &metadata,
	})
}

// 文件检查接口
func checkFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取文件名
	vars := mux.Vars(r)
	filename := vars["filename"]

	// 安全检查：防止路径遍历
	if strings.Contains(filename, "..") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid filename",
		})
		return
	}

	// 构建文件路径
	filePath := filepath.Join(config.Storage.AudioDir, filename)

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		// 文件不存在
		json.NewEncoder(w).Encode(CheckFileResponse{
			Exists:   false,
			Filename: filename,
		})
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "File check failed",
		})
		return
	}

	// 文件存在，返回文件信息
	json.NewEncoder(w).Encode(CheckFileResponse{
		Exists:     true,
		Filename:   filename,
		Size:       info.Size(),
		UploadTime: info.ModTime().Format(time.RFC3339),
	})
}

// 获取视频元数据接口
func getMetadataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取文件名
	vars := mux.Vars(r)
	filename := vars["filename"]

	// 安全检查：防止路径遍历
	if strings.Contains(filename, "..") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MetadataResponse{
			Success: false,
			Error:   "Invalid filename",
		})
		return
	}

	// 加载元数据
	metadata, err := loadMetadata(filename)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(MetadataResponse{
				Success: false,
				Error:   "Metadata not found",
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(MetadataResponse{
				Success: false,
				Error:   "Failed to load metadata",
			})
		}
		return
	}

	// 返回元数据
	json.NewEncoder(w).Encode(MetadataResponse{
		Success:  true,
		Metadata: metadata,
	})
}

// 删除文件及元数据接口
func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取文件名
	vars := mux.Vars(r)
	filename := vars["filename"]

	// 安全检查：防止路径遍历
	if strings.Contains(filename, "..") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "Invalid filename",
		})
		return
	}

	// 构建文件路径
	filePath := filepath.Join(config.Storage.AudioDir, filename)
	metaFilePath := filepath.Join(config.Storage.AudioDir, filename+".meta.json")

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		fileLogger.Printf("删除文件失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "Failed to delete file",
		})
		return
	}

	// 删除元数据文件（如果存在）
	if err := os.Remove(metaFilePath); err != nil && !os.IsNotExist(err) {
		fileLogger.Printf("警告: 删除元数据文件失败: %v", err)
		// 元数据删除失败不影响整体操作
	}

	fileLogger.Printf("文件已删除: %s", filename)

	// 添加到已删除文件记录
	deletedFilesManager.Add(filename)
	fileLogger.Printf("已添加到删除记录: %s", filename)

	// 添加到取消收藏文件记录
	uncollectedFilesManager.Add(filename)
	fileLogger.Printf("已添加到取消收藏记录: %s", filename)

	json.NewEncoder(w).Encode(DeleteFileResponse{
		Success: true,
		Message: "File deleted successfully",
	})
}

// 查询视频列表接口
func queryVideosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 解析请求体
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	fileLogger.Printf("查询视频列表: %+v", req)

	// 读取音频目录
	entries, err := os.ReadDir(config.Storage.AudioDir)
	if err != nil {
		fileLogger.Printf("读取目录失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(QueryResponse{
			Success: false,
			Error:   "Failed to read directory",
		})
		return
	}

	var videos []VideoFile
	prefix := ""
	suffix := ""

	// 获取过滤条件
	if req.Filters != nil {
		prefix = req.Filters.Prefix
		suffix = req.Filters.Suffix
	}

	// 遍历目录，收集视频文件
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// 跳过元数据文件
		if strings.HasSuffix(filename, ".meta.json") {
			continue
		}

		// 应用过滤条件
		if prefix != "" && !strings.HasPrefix(filename, prefix) {
			continue
		}
		if suffix != "" && !strings.HasSuffix(filename, suffix) {
			continue
		}

		// 获取文件信息
		info, err := entry.Info()
		if err != nil {
			fileLogger.Printf("获取文件信息失败: %s - %v", filename, err)
			continue
		}

		// 从文件名提取 ID（去掉扩展名）
		id := filename
		if ext := filepath.Ext(filename); ext != "" {
			id = filename[:len(filename)-len(ext)]
		}

		// 构建访问 URL
		url := fmt.Sprintf("/audio/%s", filename)

		videos = append(videos, VideoFile{
			ID:       id,
			Filename: filename,
			Size:     info.Size(),
			URL:      url,
		})
	}

	fileLogger.Printf("查询到 %d 个视频文件", len(videos))

	json.NewEncoder(w).Encode(QueryResponse{
		Success: true,
		Videos:  videos,
	})
}

// 下载视频接口
func downloadVideoHandler(w http.ResponseWriter, r *http.Request) {
	// 获取视频 ID
	vars := mux.Vars(r)
	id := vars["id"]

	fileLogger.Printf("下载视频: %s", id)

	// 安全检查：防止路径遍历
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		http.Error(w, "Invalid video ID", http.StatusBadRequest)
		return
	}

	// 尝试多种可能的文件扩展名
	extensions := []string{".mp4", ".MP4", ".wav", ".WAV"}
	var filePath string
	var found bool

	for _, ext := range extensions {
		testPath := filepath.Join(config.Storage.AudioDir, id+ext)
		if _, err := os.Stat(testPath); err == nil {
			filePath = testPath
			found = true
			break
		}
	}

	if !found {
		fileLogger.Printf("视频文件不存在: %s", id)
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		fileLogger.Printf("打开文件失败: %v", err)
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// 获取文件信息
	info, err := file.Stat()
	if err != nil {
		fileLogger.Printf("获取文件信息失败: %v", err)
		http.Error(w, "Failed to get file info", http.StatusInternalServerError)
		return
	}

	// 设置响应头
	contentType := "video/mp4"
	if strings.HasSuffix(filePath, ".wav") || strings.HasSuffix(filePath, ".WAV") {
		contentType = "audio/wav"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))

	// 复制文件到响应
	http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), file)

	fileLogger.Printf("视频下载成功: %s (%d bytes)", id, info.Size())
}

// 日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fileLogger.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
		fileLogger.Printf("完成耗时: %v", time.Since(start))
	})
}

// 恢复中间件（防止panic导致服务崩溃）
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fileLogger.Printf("Panic恢复: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func main() {
	// 加载配置
	if err := loadConfig(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 初始化日志
	if err := initLogger(); err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}

	// 确保音频目录存在
	if err := os.MkdirAll(config.Storage.AudioDir, 0755); err != nil {
		fileLogger.Fatalf("无法创建音频目录: %v", err)
	}

	// 初始化已删除文件管理器
	deletedFilesManager = NewDeletedFilesManager(config.Storage.AudioDir)
	fileLogger.Printf("已删除文件管理器已初始化")

	// 初始化已读文件管理器
	readFilesManager = NewReadFilesManager(config.Storage.AudioDir)
	fileLogger.Printf("已读文件管理器已初始化")

	// 初始化取消收藏文件管理器
	uncollectedFilesManager = NewUncollectedFilesManager(config.Storage.AudioDir)
	fileLogger.Printf("取消收藏文件管理器已初始化")

	// 创建路由
	r := mux.NewRouter()

	// 注册路由
	r.HandleFunc("/", healthHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("POST")
	r.HandleFunc("/api/check/{filename}", checkFileHandler).Methods("GET")
	r.HandleFunc("/api/metadata/{filename}", getMetadataHandler).Methods("GET")
	r.HandleFunc("/api/file/{filename}", deleteFileHandler).Methods("DELETE")
	r.HandleFunc("/api/videos/{filename}", deleteFileHandler).Methods("DELETE")
	r.HandleFunc("/api/videos/query", queryVideosHandler).Methods("POST")
	r.HandleFunc("/api/videos/{id}/download", downloadVideoHandler).Methods("GET")

	// 已删除文件管理接口
	r.HandleFunc("/api/deleted/files", getDeletedFilesHandler).Methods("GET")
	r.HandleFunc("/api/deleted/check", checkDeletedFilesHandler).Methods("POST")
	r.HandleFunc("/api/deleted/cleanup", cleanupDeletedRecordsHandler).Methods("POST")

	// 已读文件管理接口
	r.HandleFunc("/api/read/mark", markReadHandler).Methods("POST")
	r.HandleFunc("/api/read/files", getReadFilesHandler).Methods("GET")
	r.HandleFunc("/api/read/remove", removeReadRecordHandler).Methods("DELETE")

	// 取消收藏文件管理接口
	r.HandleFunc("/api/uncollected/files", getUncollectedFilesHandler).Methods("GET")
	r.HandleFunc("/api/uncollected/remove", removeUncollectedRecordHandler).Methods("DELETE")

	// 静态文件服务
	r.PathPrefix("/audio/").Handler(http.StripPrefix("/audio/", http.FileServer(http.Dir(config.Storage.AudioDir))))

	// 应用中间件
	handler := recoveryMiddleware(loggingMiddleware(r))

	// 启动服务器
	addr := ":" + config.Server.Port
	fileLogger.Println("🚀 音频文件服务器启动成功!")
	fileLogger.Printf("📁 音频目录: %s", config.Storage.AudioDir)
	fileLogger.Printf("🌐 监听地址: 0.0.0.0:%s", config.Server.Port)
	fileLogger.Printf("✅ 健康检查: http://localhost:%s/", config.Server.Port)
	fileLogger.Printf("📤 上传接口: http://localhost:%s/upload", config.Server.Port)
	fileLogger.Printf("🔍 文件检查: http://localhost:%s/api/check/{filename}", config.Server.Port)

	if err := http.ListenAndServe(addr, handler); err != nil {
		fileLogger.Fatalf("服务器启动失败: %v", err)
	}
}
