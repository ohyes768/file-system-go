# file-system-go API 接口文档

## 概述

file-system-go 为 douyin-collector 提供文件存储服务接口。本文档描述 API 规范。

## 版本历史

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| 1.1.0 | 2026-02-26 | 添加文件检查接口 |
| 1.0.0 | 2025-01-28 | 初始版本 |

---

## 1. 文件检查接口（新增）

### 1.1 检查文件是否存在

**接口描述**：检查服务器上是否已存在指定文件

**请求**

```http
GET /api/check/{filename}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filename | string | 是 | 文件名（如：7123456789012345678.mp4） |

**响应示例**

文件存在：
```json
{
  "exists": true,
  "filename": "7123456789012345678.mp4",
  "size": 102400000,
  "upload_time": "2026-02-26T12:00:00Z"
}
```

文件不存在：
```json
{
  "exists": false,
  "filename": "7123456789012345678.mp4"
}
```

**响应字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| exists | boolean | 文件是否存在 |
| filename | string | 文件名 |
| size | number | 文件大小（字节），仅 exists=true 时返回 |
| upload_time | string | 上传时间（ISO 8601），仅 exists=true 时返回 |

---

## 2. 文件上传接口（已有）

### 2.1 上传视频文件

**接口描述**：上传视频文件到服务器

**请求**

```http
POST /api/upload
Content-Type: multipart/form-data
```

**表单参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | File | 是 | 视频文件（MP4 格式） |

**成功响应**

```json
{
  "success": true,
  "filename": "7123456789012345678.mp4",
  "url": "/audio/7123456789012345678.mp4",
  "size": 102400000
}
```

**失败响应**

```json
{
  "success": false,
  "error": "错误描述"
}
```

**响应字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| success | boolean | 上传是否成功 |
| filename | string | 保存的文件名 |
| url | string | 文件访问 URL |
| size | number | 文件大小（字节） |
| error | string | 错误描述，仅 success=false 时返回 |

---

## 3. 实现要求

### 3.1 新增接口

**GET /api/check/{filename}** - 文件检查接口

需要实现以下功能：
1. 检查文件存储目录中是否存在指定文件
2. 如果存在，返回文件信息（大小、修改时间）
3. 如果不存在，返回 exists=false

### 3.2 接口列表

| 接口 | 状态 | 说明 |
|------|------|------|
| GET / | ✅ 已有 | 健康检查 |
| GET /audio/{filename} | ✅ 已有 | 静态文件访问 |
| POST /api/upload | ✅ 已有 | 文件上传 |
| GET /api/check/{filename} | 🆕 需添加 | 文件检查 |

---

## 4. Go 代码实现参考

### 4.1 文件检查接口

```go
// 文件检查请求处理器
func checkFileHandler(w http.ResponseWriter, r *http.Request) {
    // 获取文件名
    filename := mux.Vars(r)["filename"]

    // 安全检查：防止路径遍历
    if strings.Contains(filename, "..") {
        http.Error(w, "Invalid filename", http.StatusBadRequest)
        return
    }

    // 构建文件路径
    filepath := path.Join(config.AudioDir, filename)

    // 检查文件是否存在
    info, err := os.Stat(filepath)
    if os.IsNotExist(err) {
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]interface{}{
            "exists": false,
            "filename": filename,
        })
        return
    }

    if err != nil {
        http.Error(w, "File check failed", http.StatusInternalServerError)
        return
    }

    // 返回文件信息
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "exists": true,
        "filename": filename,
        "size": info.Size(),
        "upload_time": info.ModTime().Format(time.RFC3339),
    })
}
```

### 4.2 路由注册

```go
func main() {
    r := mux.NewRouter()

    // 现有路由
    r.HandleFunc("/", healthHandler).Methods("GET")
    r.HandleFunc("/upload", uploadHandler).Methods("POST")
    r.PathPrefix("/audio/").Handler(http.StripPrefix("/audio/", http.FileServer(http.Dir(audioDir))))

    // 新增：文件检查接口
    r.HandleFunc("/api/check/{filename}", checkFileHandler).Methods("GET")

    // 启动服务器
    http.ListenAndServe(":8000", r)
}
```

---

## 5. 测试

### 5.1 文件存在

```bash
curl http://localhost:8000/api/check/7123456789012345678.mp4
```

**预期响应**
```json
{
  "exists": true,
  "filename": "7123456789012345678.mp4",
  "size": 102400000,
  "upload_time": "2026-02-26T12:00:00Z"
}
```

### 5.2 文件不存在

```bash
curl http://localhost:8000/api/check/nonexistent.mp4
```

**预期响应**
```json
{
  "exists": false,
  "filename": "nonexistent.mp4"
}
```

---

## 附录

### A. 相关文档

- [技术规范文档](技术规范文档.md)
- [ECS文件服务器部署指南](ECS文件服务器部署指南.md)
- [douyin-collector API 规范](../douyin-collector/docs/API接口文档.md)

### B. 变更记录

- 2026-02-26: 添加文件检查接口需求
