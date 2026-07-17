package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
)

var (
	fileLogger    *log.Logger
	consoleLogger = log.New(os.Stdout, "", log.LstdFlags)
)

func initLogger() error {
	if err := os.MkdirAll(config.Logging.LogDir, 0755); err != nil {
		return fmt.Errorf("无法创建日志目录: %v", err)
	}

	dateStr := time.Now().Format("2006-01-02")
	logFile := filepath.Join(config.Logging.LogDir, fmt.Sprintf("audio-server-%s.log", dateStr))

	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("无法创建日志文件: %v", err)
	}

	multiWriter := io.MultiWriter(os.Stdout, file)
	fileLogger = log.New(multiWriter, "", log.LstdFlags)

	return nil
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fileLogger.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
		fileLogger.Printf("完成耗时: %v", time.Since(start))
	})
}

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
	if err := loadConfig(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	if err := initLogger(); err != nil {
		log.Fatalf("日志初始化失败: %v", err)
	}

	if err := os.MkdirAll(config.Storage.AudioDir, 0755); err != nil {
		fileLogger.Fatalf("无法创建音频目录: %v", err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/api/files", uploadHandler).Methods("POST")
	r.HandleFunc("/api/files/query", queryVideosHandler).Methods("POST")
	r.HandleFunc("/api/files/list", listFilesHandler).Methods("GET")
	r.HandleFunc("/api/files/dir/{name}", deleteDirHandler).Methods("DELETE")
	r.HandleFunc("/api/files/{id}/download", downloadVideoHandler).Methods("GET")
	r.HandleFunc("/api/files/{id}", deleteFileHandler).Methods("DELETE")

	r.PathPrefix("/audio/").Handler(http.StripPrefix("/audio/", http.FileServer(http.Dir(config.Storage.AudioDir))))

	handler := recoveryMiddleware(loggingMiddleware(r))

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
