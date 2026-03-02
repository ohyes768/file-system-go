package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type DeletedFilesListResponse struct {
	Success bool                `json:"success"`
	Records []DeletedFileRecord `json:"records,omitempty"`
	Error   string              `json:"error,omitempty"`
}

type CheckDeletedRequest struct {
	Filenames []string `json:"filenames"`
}

type CheckDeletedResponse struct {
	Success bool     `json:"success"`
	Deleted []string `json:"deleted,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// 查询已删除文件列表
func getDeletedFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(DeletedFilesListResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	records := deletedFilesManager.GetAll()

	json.NewEncoder(w).Encode(DeletedFilesListResponse{
		Success: true,
		Records: records,
	})
}

// 批量检查文件是否已删除
func checkDeletedFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CheckDeletedResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	var req CheckDeletedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CheckDeletedResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	deleted := make([]string, 0)
	for _, filename := range req.Filenames {
		if deletedFilesManager.IsDeleted(filename) {
			deleted = append(deleted, filename)
		}
	}

	json.NewEncoder(w).Encode(CheckDeletedResponse{
		Success: true,
		Deleted: deleted,
	})
}

// 清理超过指定天数的删除记录
func cleanupDeletedRecordsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 默认清理 30 天前的记录
	days := 30

	// 可以从查询参数获取天数
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	count := deletedFilesManager.Cleanup(days)

	fileLogger.Printf("已清理 %d 条超过 %d 天的删除记录", count, days)

	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: "已完成",
	})
}
