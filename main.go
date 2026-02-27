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
	Success bool   `json:"success"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Error    string `json:"error,omitempty"`
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

var (
	config      Config
	fileLogger  *log.Logger
	consoleLogger = log.New(os.Stdout, "", log.LstdFlags)
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

// 健康检查接口
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{
		Service:        "Audio File Server (Go)",
		Status:         "running",
		Version:        "1.1.0",
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
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "文件过大或解析失败",
		})
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "未找到上传文件",
		})
		return
	}
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
	fileURL := fmt.Sprintf("/audio/%s", filename)

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(UploadResponse{
		Success: true,
		Filename: filename,
		URL:      fileURL,
		Size:     size,
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
	r.HandleFunc("/", healthHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("POST")
	r.HandleFunc("/api/check/{filename}", checkFileHandler).Methods("GET")

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
