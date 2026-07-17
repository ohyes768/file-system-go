package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type UploadResponse struct {
	Success      bool   `json:"success"`
	Filename     string `json:"filename,omitempty"`
	URL          string `json:"url,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ExtractedDir string `json:"extracted_dir,omitempty"`
	Error        string `json:"error,omitempty"`
}

type HealthResponse struct {
	Service  string `json:"service"`
	Status   string `json:"status"`
	Version  string `json:"version"`
	AudioDir string `json:"audio_dir"`
}

type DeleteFileResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type VideoFile struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	URL      string `json:"url,omitempty"`
}

type QueryRequest struct {
	Filters *QueryFilters `json:"filters,omitempty"`
}

type QueryFilters struct {
	Prefix string `json:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty"`
}

type QueryResponse struct {
	Success bool        `json:"success"`
	Videos  []VideoFile `json:"videos,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type ListEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
	URL  string `json:"url,omitempty"`
}

type ListResponse struct {
	Success bool        `json:"success"`
	Path    string      `json:"path"`
	Entries []ListEntry `json:"entries,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type DeleteDirResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

func invalidName(name string) bool {
	return strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\")
}

func resolveStoragePath(rel string) (string, error) {
	if rel == "" {
		return config.Storage.AudioDir, nil
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid path")
	}
	rel = filepath.ToSlash(rel)
	rel = strings.Trim(rel, "/")
	if rel == "" {
		return config.Storage.AudioDir, nil
	}
	full := filepath.Join(config.Storage.AudioDir, filepath.FromSlash(rel))
	absBase, err := filepath.Abs(config.Storage.AudioDir)
	if err != nil {
		return "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if absFull != absBase && !strings.HasPrefix(absFull, absBase+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid path")
	}
	return full, nil
}

func audioURL(pathPrefix, name string) string {
	if pathPrefix == "" {
		return "/audio/" + name
	}
	return "/audio/" + pathPrefix + "/" + name
}

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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	r.Body = http.MaxBytesReader(w, r.Body, int64(config.Storage.MaxUploadMB)<<20)

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

	filename := header.Filename
	filePath := filepath.Join(config.Storage.AudioDir, filename)

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

	fileURL := fmt.Sprintf("/api/files/%s/download", filename)
	resp := UploadResponse{
		Success:  true,
		Filename: filename,
		URL:      fileURL,
		Size:     size,
	}

	extract := r.FormValue("extract") == "true" || r.URL.Query().Get("extract") == "true"
	if extract {
		if base := ArchiveBaseName(filename); base != "" {
			dest := filepath.Join(config.Storage.AudioDir, base)
			if err := os.MkdirAll(dest, 0755); err != nil {
				fileLogger.Printf("创建解压目录失败: %v", err)
				resp.Error = fmt.Sprintf("解压失败: %v", err)
			} else if err := ExtractArchive(filePath, dest); err != nil {
				fileLogger.Printf("解压失败: %v", err)
				resp.Error = fmt.Sprintf("解压失败: %v", err)
			} else {
				resp.ExtractedDir = base
				fileLogger.Printf("解压成功: %s -> %s", filename, base)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id := vars["id"]

	if invalidName(id) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "Invalid file id",
		})
		return
	}

	filePath := filepath.Join(config.Storage.AudioDir, id)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(DeleteFileResponse{
			Success: false,
			Error:   "File not found",
		})
		return
	}

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

func deleteDirHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	name := vars["name"]

	if invalidName(name) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DeleteDirResponse{
			Success: false,
			Error:   "Invalid directory name",
		})
		return
	}

	dirPath := filepath.Join(config.Storage.AudioDir, name)
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(DeleteDirResponse{
			Success: false,
			Error:   "Directory not found",
		})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteDirResponse{
			Success: false,
			Error:   "Failed to stat directory",
		})
		return
	}
	if !info.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DeleteDirResponse{
			Success: false,
			Error:   "Not a directory",
		})
		return
	}

	if err := os.RemoveAll(dirPath); err != nil {
		fileLogger.Printf("删除目录失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteDirResponse{
			Success: false,
			Error:   "Failed to delete directory",
		})
		return
	}

	fileLogger.Printf("目录已删除: %s", name)

	json.NewEncoder(w).Encode(DeleteDirResponse{
		Success: true,
		Message: "Directory deleted successfully",
	})
}

func queryVideosHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

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

	if req.Filters != nil {
		prefix = req.Filters.Prefix
		suffix = req.Filters.Suffix
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		if prefix != "" && !strings.HasPrefix(filename, prefix) {
			continue
		}
		if suffix != "" && !strings.HasSuffix(filename, suffix) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			fileLogger.Printf("获取文件信息失败: %s - %v", filename, err)
			continue
		}

		id := filename
		if ext := filepath.Ext(filename); ext != "" {
			id = filename[:len(filename)-len(ext)]
		}

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

func listFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	pathParam := r.URL.Query().Get("path")
	prefix := r.URL.Query().Get("prefix")

	dirPath, err := resolveStoragePath(pathParam)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ListResponse{
			Success: false,
			Error:   "Invalid path",
		})
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ListResponse{
				Success: false,
				Error:   "Path not found",
			})
			return
		}
		fileLogger.Printf("读取目录失败: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ListResponse{
			Success: false,
			Error:   "Failed to read directory",
		})
		return
	}

	normalizedPath := filepath.ToSlash(strings.Trim(pathParam, "/"))
	var listEntries []ListEntry
	for _, entry := range entries {
		name := entry.Name()
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			fileLogger.Printf("获取条目信息失败: %s - %v", name, err)
			continue
		}

		if entry.IsDir() {
			listEntries = append(listEntries, ListEntry{
				Name: name,
				Type: "dir",
				Size: 0,
				URL:  "",
			})
			continue
		}

		listEntries = append(listEntries, ListEntry{
			Name: name,
			Type: "file",
			Size: info.Size(),
			URL:  audioURL(normalizedPath, name),
		})
	}

	json.NewEncoder(w).Encode(ListResponse{
		Success: true,
		Path:    normalizedPath,
		Entries: listEntries,
	})
}

func downloadVideoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	fileLogger.Printf("下载文件: %s", id)

	if invalidName(id) {
		http.Error(w, "Invalid file id", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(config.Storage.AudioDir, id)

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

	file, err := os.Open(filePath)
	if err != nil {
		fileLogger.Printf("打开文件失败: %v", err)
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

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

	http.ServeContent(w, r, filepath.Base(filePath), info.ModTime(), file)

	fileLogger.Printf("文件下载成功: %s (%d bytes)", id, info.Size())
}
