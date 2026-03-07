package main

import (
	"encoding/json"
	"net/http"
)

// MarkReadRequest 标记已读请求
type MarkReadRequest struct {
	Filename string `json:"filename"` // 必填，如 "xxx.wav"
}

// MarkReadResponse 标记已读响应
type MarkReadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ReadFilesListResponse 已读文件列表响应
type ReadFilesListResponse struct {
	Success bool             `json:"success"`
	Records []ReadFileRecord `json:"records,omitempty"`
	Error   string           `json:"error,omitempty"`
}

// markReadHandler 标记文件为已读
func markReadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(MarkReadResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	var req MarkReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MarkReadResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.Filename == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MarkReadResponse{
			Success: false,
			Error:   "Filename is required",
		})
		return
	}

	readFilesManager.Mark(req.Filename)

	fileLogger.Printf("文件已标记为已读: %s", req.Filename)

	json.NewEncoder(w).Encode(MarkReadResponse{
		Success: true,
		Message: "已标记为已读",
	})
}

// getReadFilesHandler 查询已读文件列表
func getReadFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ReadFilesListResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// 支持按 filename 精确匹配查询
	filename := r.URL.Query().Get("filename")

	var records []ReadFileRecord
	if filename != "" {
		// 按文件名精确匹配
		if record, ok := readFilesManager.GetByFilename(filename); ok {
			records = append(records, *record)
		}
	} else {
		// 获取所有记录
		records = readFilesManager.GetAll()
	}

	json.NewEncoder(w).Encode(ReadFilesListResponse{
		Success: true,
		Records: records,
	})
}