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
)

// 响应结构体
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type UploadResponse struct {
	Success  bool   `json:"success"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Error    string `json:"error,omitempty"`
}

type HealthResponse struct {
	Service  string `json:"service"`
	Status   string `json:"status"`
	Version  string `json:"version"`
	AudioDir string `json:"audio_dir"`
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
	fileLogger    *log.Logger
	consoleLogger = log.New(os.Stdout, "", log.LstdFlags)
)

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

// 健康检查接口
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{
		Service:  "File Server (Go)",
		Status:   "running",
		Version:  "2.0.0",
		AudioDir: config.Storage.AudioDir,
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

	// 构建访问 URL
	fileURL := fmt.Sprintf("/api/files/%s/download", filename)

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(UploadResponse{
		Success:  true,
		Filename: filename,
		URL:      fileURL,
		Size:     size,
	})
}

// 删除文件接口
func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 获取文件 ID（即客户端上传时的文件名）
	vars := mux.Vars(r)
	id := vars["id"]

	// 安全检查：防止路径遍历
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "Invalid file id",
		})
		return
	}

	// 构建文件路径
	filePath := filepath.Join(config.Storage.AudioDir, id)

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

	// 直接 hard delete
	if err := os.Remove(filePath); err != nil {
		fileLogger.Printf("删除文件失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "Failed to delete file",
		})
		return
	}

	fileLogger.Printf("文件已删除: %s", id)

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

// 下载文件接口
func downloadVideoHandler(w http.ResponseWriter, r *http.Request) {
	// 获取文件 ID（即上传时的客户端文件名）
	vars := mux.Vars(r)
	id := vars["id"]

	fileLogger.Printf("下载文件: %s", id)

	// 安全检查：防止路径遍历
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		http.Error(w, "Invalid file id", http.StatusBadRequest)
		return
	}

	// ID = 完整文件名（含扩展名），直接定位
	filePath := filepath.Join(config.Storage.AudioDir, id)

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		fileLogger.Printf("文件不存在: %s", id)
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	if err != nil {
		fileLogger.Printf("获取文件信息失败: %v", err)
		http.Error(w, "Failed to get file info", http.StatusInternalServerError)
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

	// 设置响应头
	contentType := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".mp4":
		contentType = "video/mp4"
	case ".wav":
		contentType = "audio/wav"
	case ".mp3":
		contentType = "audio/mpeg"
	case ".m4a":
		contentType = "audio/mp4"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))

	// 复制文件到响应
	http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), file)

	fileLogger.Printf("文件下载成功: %s (%d bytes)", id, info.Size())
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

	// 创建路由
	r := mux.NewRouter()

	// 注册路由
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/files", uploadHandler).Methods("POST")
	r.HandleFunc("/api/files/query", queryVideosHandler).Methods("POST")
	r.HandleFunc("/api/files/{id}/download", downloadVideoHandler).Methods("GET")
	r.HandleFunc("/api/files/{id}", deleteFileHandler).Methods("DELETE")

	// 静态文件服务
	r.PathPrefix("/audio/").Handler(http.StripPrefix("/audio/", http.FileServer(http.Dir(config.Storage.AudioDir))))

	// 应用中间件
	handler := recoveryMiddleware(loggingMiddleware(r))

	// 启动服务器
	addr := ":" + config.Server.Port
	fileLogger.Println("🚀 文件服务器启动成功!")
	fileLogger.Printf("📁 文件目录: %s", config.Storage.AudioDir)
	fileLogger.Printf("🌐 监听地址: 0.0.0.0:%s", config.Server.Port)
	fileLogger.Printf("✅ 健康检查: http://localhost:%s/health", config.Server.Port)
	fileLogger.Printf("📤 上传接口: http://localhost:%s/api/files", config.Server.Port)

	if err := http.ListenAndServe(addr, handler); err != nil {
		fileLogger.Fatalf("服务器启动失败: %v", err)
	}
}
