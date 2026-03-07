package main

import (
	"encoding/json"
	"net/http"
)

// UncollectedFilesListResponse 取消收藏文件列表响应
type UncollectedFilesListResponse struct {
	Success bool                  `json:"success"`
	Records []UncollectedFileRecord `json:"records,omitempty"`
	Error   string                `json:"error,omitempty"`
}

// RemoveUncollectedRequest 删除取消收藏记录请求
type RemoveUncollectedRequest struct {
	Filename string `json:"filename"`
}

// RemoveUncollectedResponse 删除取消收藏记录响应
type RemoveUncollectedResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// getUncollectedFilesHandler 获取取消收藏文件列表
func getUncollectedFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(UncollectedFilesListResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	records := uncollectedFilesManager.GetAll()

	json.NewEncoder(w).Encode(UncollectedFilesListResponse{
		Success: true,
		Records: records,
	})
}

// removeUncollectedRecordHandler 删除取消收藏记录
func removeUncollectedRecordHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(RemoveUncollectedResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	var req RemoveUncollectedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RemoveUncollectedResponse{
			Success: false,
			Error:   "Invalid request body",
		})
		return
	}

	if req.Filename == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(RemoveUncollectedResponse{
			Success: false,
			Error:   "Filename is required",
		})
		return
	}

	removed := uncollectedFilesManager.Remove(req.Filename)

	if removed {
		fileLogger.Printf("已移除取消收藏记录: %s", req.Filename)
		json.NewEncoder(w).Encode(RemoveUncollectedResponse{
			Success: true,
			Message: "记录已移除",
		})
	} else {
		fileLogger.Printf("未找到取消收藏记录: %s", req.Filename)
		json.NewEncoder(w).Encode(RemoveUncollectedResponse{
			Success: false,
			Error:   "Record not found",
		})
	}
}