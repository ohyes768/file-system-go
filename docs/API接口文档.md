# file-system-go API 接口文档

## 概述

file-system-go 为 douyin-collector 提供文件存储服务接口。本文档描述 API 规范。

## 版本历史

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| 1.4.0 | 2026-03-02 | 添加已删除文件记录功能，防止重复推送 |
| 1.3.0 | 2026-02-28 | 添加视频列表查询和下载接口 |
| 1.2.0 | 2026-02-27 | 添加视频元数据支持（标题、作者、描述） |
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

## 2. 视频元数据接口（v1.2.0 新增）

### 2.1 获取视频元数据

**接口描述**：获取指定视频的元数据信息

**请求**

```http
GET /api/metadata/{filename}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filename | string | 是 | 文件名（如：video.mp4） |

**成功响应**

```json
{
  "success": true,
  "metadata": {
    "filename": "video.mp4",
    "title": "视频标题",
    "author": "作者名称",
    "description": "视频描述",
    "upload_time": "2026-02-27T12:00:00+08:00"
  }
}
```

**失败响应**

元数据不存在：
```json
{
  "success": false,
  "error": "Metadata not found"
}
```

### 2.2 删除视频及元数据

**接口描述**：删除视频文件及其关联的元数据

**请求**

```http
DELETE /api/file/{filename}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filename | string | 是 | 文件名（如：video.mp4） |

**成功响应**

```json
{
  "success": true,
  "message": "File deleted successfully"
}
```

**失败响应**

文件不存在：
```json
{
  "success": false,
  "error": "File not found"
}
```

### 2.3 元数据存储格式

元数据以 JSON 伴生文件形式存储，文件名为 `{filename}.meta.json`：

```
audio_files/
├── video1.mp4
├── video1.mp4.meta.json
├── video2.mp4
└── video2.mp4.meta.json
```

**元数据文件内容示例**：

```json
{
  "filename": "video1.mp4",
  "title": "精彩视频合集",
  "author": "创作者名称",
  "description": "这是一个精彩视频的描述",
  "upload_time": "2026-02-27T12:00:00+08:00"
}
```

---

## 3. 视频查询和下载接口（v1.3.0 新增）

### 3.1 查询视频列表

**接口描述**：查询服务器上的视频文件列表，支持前缀和后缀过滤

**请求**

```http
POST /api/videos/query
Content-Type: application/json
```

**请求体**

```json
{
  "filters": {
    "prefix": "video",
    "suffix": ".mp4"
  }
}
```

**请求字段说明**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filters | object | 否 | 过滤条件对象 |
| filters.prefix | string | 否 | 文件名前缀过滤 |
| filters.suffix | string | 否 | 文件名后缀过滤（如 .mp4） |

**成功响应**

```json
{
  "success": true,
  "videos": [
    {
      "id": "video1",
      "filename": "video1.mp4",
      "size": 102400000,
      "url": "/audio/video1.mp4"
    },
    {
      "id": "video2",
      "filename": "video2.mp4",
      "size": 204800000,
      "url": "/audio/video2.mp4"
    }
  ]
}
```

**失败响应**

```json
{
  "success": false,
  "error": "Failed to read directory"
}
```

**响应字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| success | boolean | 查询是否成功 |
| videos | array | 视频文件列表 |
| videos[].id | string | 视频 ID（文件名去掉扩展名） |
| videos[].filename | string | 完整文件名 |
| videos[].size | number | 文件大小（字节） |
| videos[].url | string | 文件访问 URL |
| error | string | 错误描述，仅 success=false 时返回 |

**请求示例（curl）**

```bash
# 查询所有视频
curl -X POST http://localhost:8000/api/videos/query

# 查询所有 .mp4 文件
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"suffix": ".mp4"}}' \
  http://localhost:8000/api/videos/query

# 查询前缀为 "test" 的 .mp4 文件
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"prefix": "test", "suffix": ".mp4"}}' \
  http://localhost:8000/api/videos/query
```

### 3.2 下载视频

**接口描述**：根据视频 ID 下载视频文件

**请求**

```http
GET /api/videos/{id}/download
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 视频 ID（文件名去掉扩展名） |

**支持的文件扩展名**

- .mp4 / .MP4（视频文件）
- .wav / .WAV（音频文件）

**成功响应**

返回文件二进制内容，响应头包含：

| 响应头 | 说明 |
|--------|------|
| Content-Type | video/mp4 或 audio/wav |
| Content-Length | 文件大小（字节） |
| Content-Disposition | attachment; filename="{filename}" |

**失败响应**

- **400 Bad Request**: 无效的视频 ID（包含路径遍历字符）
- **404 Not Found**: 视频文件不存在

**请求示例（curl）**

```bash
# 下载视频
curl -O http://localhost:8000/api/videos/video1/download

# 下载并指定输出文件名
curl http://localhost:8000/api/videos/video1/download --output my-video.mp4
```

---

## 4. 文件上传接口（更新）

### 4.1 上传视频文件（支持元数据）

```
audio_files/
├── video1.mp4
├── video1.mp4.meta.json
├── video2.mp4
└── video2.mp4.meta.json
```

**元数据文件内容示例**：

```json
{
  "filename": "video1.mp4",
  "title": "精彩视频合集",
  "author": "创作者名称",
  "description": "这是一个精彩视频的描述",
  "upload_time": "2026-02-27T12:00:00+08:00"
}
```

---

## 3. 文件上传接口（更新）

### 3.1 上传视频文件（支持元数据）

**接口描述**：上传视频文件到服务器，支持传递标题、作者、描述等元数据

**请求**

```http
POST /upload
Content-Type: multipart/form-data
```

**表单参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | File | 是 | 视频文件（MP4 格式） |
| title | string | 否 | 视频标题 |
| author | string | 否 | 作者名称 |
| description | string | 否 | 视频描述 |

**成功响应**

```json
{
  "success": true,
  "filename": "video.mp4",
  "url": "/audio/video.mp4",
  "size": 102400000,
  "metadata": {
    "filename": "video.mp4",
    "title": "视频标题",
    "author": "作者名称",
    "description": "视频描述",
    "upload_time": "2026-02-27T12:00:00+08:00"
  }
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
| metadata | object | 元数据信息，v1.2.0 新增 |
| error | string | 错误描述，仅 success=false 时返回 |

**请求示例（curl）**

```bash
curl -X POST \
  -F "file=@video.mp4" \
  -F "title=精彩视频" \
  -F "author=创作者" \
  -F "description=这是一个精彩视频" \
  http://localhost:8000/upload
```

---

## 5. 文件检查接口（已有）

---

## 6. 接口列表

### 6.1 所有接口

| 接口 | 方法 | 状态 | 说明 |
|------|------|------|------|
| GET / | ✅ 已有 | 健康检查 |
| POST /upload | ✅ 已有 | 文件上传（v1.2.0 支持元数据） |
| GET /audio/{filename} | ✅ 已有 | 静态文件访问 |
| GET /api/check/{filename} | ✅ 已有 | 文件检查 |
| GET /api/metadata/{filename} | ✅ v1.2.0 | 获取视频元数据 |
| DELETE /api/file/{filename} | ✅ v1.2.0 | 删除视频及元数据 |
| POST /api/videos/query | ✅ v1.3.0 | 查询视频列表 |
| GET /api/videos/{id}/download | ✅ v1.3.0 | 下载视频 |

### 6.2 版本对比

| 功能 | v1.0.0 | v1.1.0 | v1.2.0 | v1.3.0 |
|------|--------|--------|--------|--------|
| 文件上传 | ✅ | ✅ | ✅ | ✅ |
| 文件访问 | ✅ | ✅ | ✅ | ✅ |
| 健康检查 | ✅ | ✅ | ✅ | ✅ |
| 文件检查 | - | ✅ | ✅ | ✅ |
| 元数据上传 | - | - | ✅ | ✅ |
| 元数据查询 | - | - | ✅ | ✅ |
| 文件删除 | - | - | ✅ | ✅ |
| 视频列表查询 | - | - | - | ✅ |
| 视频下载 | - | - | - | ✅ |

---

## 7. Go 代码实现参考

### 7.1 文件检查接口

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

### 7.2 路由注册

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

## 8. 测试

### 8.1 文件存在

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

### 8.2 文件不存在

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

### 8.3 查询视频列表

```bash
# 查询所有视频
curl -X POST http://localhost:8000/api/videos/query
```

**预期响应**
```json
{
  "success": true,
  "videos": [
    {
      "id": "video1",
      "filename": "video1.mp4",
      "size": 102400000,
      "url": "/audio/video1.mp4"
    }
  ]
}
```

### 8.4 下载视频

```bash
# 下载视频
curl -O http://localhost:8000/api/videos/video1/download
```

预期响应：文件下载到当前目录，文件名为 `video1`

---

## 附录

### A. 相关文档

- [技术规范文档](技术规范文档.md)
- [ECS文件服务器部署指南](ECS文件服务器部署指南.md)
- [douyin-collector API 规范](../douyin-collector/docs/API接口文档.md)

### B. 变更记录

- 2026-02-26: 添加文件检查接口需求

---

## 9. 已删除文件管理接口（v1.4.0 新增）

### 9.1 功能说明

当用户在前端删除视频时，系统会自动将文件名记录到 `deleted_files.json`。douyin-collector 在上传前会检查该列表，避免重复推送已删除的文件。

**存储位置**：`deleted_files.json` 存储在 audio_dir 的父目录

### 9.2 查询已删除文件列表

GET /api/deleted/files

### 9.3 批量检查文件是否已删除

POST /api/deleted/check

请求体示例：
```json
{
  "filenames": ["file1.wav", "file2.wav"]
}
```

### 9.4 清理过期的删除记录

POST /api/deleted/cleanup?days=30

