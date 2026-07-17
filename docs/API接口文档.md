# file-system-go API 接口文档

## 概述

file-system-go 是一个**纯文件存储服务**，为上层应用（douyin-processor 等）提供文件存取能力。

**职责边界**：
- ✅ 管文件：上传、下载、删除、列表
- ❌ 不管业务元数据：标题/作者/描述等元数据由上层应用维护
- ❌ 不管用户状态：已读/已删/收藏等用户行为由上层应用维护

**v2.0.0 重大变更**：剥离所有业务接口（v1.4/v1.5 已读/已删/取消收藏），重构为纯文件存储。详见 [README.md §接口对照表](../README.md#接口对照表)。

## 版本历史

| 版本 | 日期 | 变更内容 |
|------|------|----------|
| 2.1.0 | 2026-07-17 | 新增 Web UI 登录、目录列表/删除、上传可选解压（`extract=true`）；**未传 extract 时上传行为与 v2.0.0 完全一致** |
| 2.0.0 | 2026-06-08 | **重大重构**。剥离元数据/已读/已删/取消收藏业务接口；统一路由为 `/api/files`；删除走 hard delete；`/` 改为 `/health`；版本号 bump |
| 1.5.0 | 2026-03-07 | 添加已读文件和取消收藏文件记录管理功能（**v2.0.0 已移除**） |
| 1.4.0 | 2026-03-02 | 添加已删除文件记录功能（**v2.0.0 已移除**） |
| 1.3.0 | 2026-02-28 | 添加视频列表查询和下载接口 |
| 1.2.0 | 2026-02-27 | 添加视频元数据支持（**v2.0.0 已移除**） |
| 1.1.0 | 2026-02-26 | 添加文件检查接口（**v2.0.0 已移除**） |
| 1.0.0 | 2025-01-28 | 初始版本 |

---

## 接口总览

| 方法 | 路径 | 作用 |
|------|------|------|
| `GET` | `/health` | 健康检查 |
| `POST` | `/api/files` | 上传文件 |
| `GET` | `/api/files/{id}/download` | 下载文件（支持 HTTP Range） |
| `DELETE` | `/api/files/{id}` | 删除文件（硬删除） |
| `POST` | `/api/files/query` | 文件列表（支持 prefix/suffix 过滤） |
| `GET` | `/api/files/list` | 目录浏览 + 前缀过滤（UI 使用，**v2.1.0 新增**） |
| `DELETE` | `/api/files/dir/{name}` | 删除子目录（**v2.1.0 新增**） |
| `POST` | `/api/ui/login` | Web UI 登录（**v2.1.0 新增**） |
| `POST` | `/api/ui/logout` | Web UI 登出（**v2.1.0 新增**） |
| `GET` | `/audio/{filename}` | 静态文件直连（旁路） |

> **文件 ID** = 上传时的客户端文件名（含扩展名），如 `video123.mp4`。

---

## 1. 健康检查

**接口描述**：获取服务运行状态、版本号、存储目录。

**请求**

```http
GET /health
```

**成功响应** (200)

```json
{
  "service": "File Server (Go)",
  "status": "running",
  "version": "2.0.0",
  "audio_dir": "./audio_files"
}
```

---

## 2. 上传文件

**接口描述**：上传一个文件到服务器。**仅接收文件，不接收业务元数据**。

**请求**

```http
POST /api/files
Content-Type: multipart/form-data
```

**表单参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | File | 是 | 文件内容（任意类型，最大 `config.max_upload_mb`） |

> ⚠️ v1.x 中的 `title` / `author` / `description` 字段已废弃，传入将被忽略。

**成功响应** (200)

```json
{
  "success": true,
  "filename": "video123.mp4",
  "url": "/api/files/video123.mp4/download",
  "size": 12345
}
```

**失败响应**

| 状态码 | 含义 |
|--------|------|
| 400 Bad Request | 文件过大 / 表单解析失败 / 未找到文件字段 |
| 500 Internal Server Error | 文件创建/写入失败 |

```json
{
  "success": false,
  "error": "错误描述"
}
```

**请求示例**

```bash
curl -X POST \
  -F "file=@video123.mp4" \
  http://localhost:8000/api/files
```

---

## 3. 下载文件

**接口描述**：根据文件 ID 下载文件。**支持 HTTP Range 协议**，可断点续传。

**请求**

```http
GET /api/files/{id}/download
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 文件 ID（= 上传时的客户端文件名，含扩展名） |

**成功响应** (200 / 206 Partial Content)

文件二进制流，响应头：

| 响应头 | 说明 |
|--------|------|
| `Content-Type` | 按扩展名推断：`video/mp4` / `audio/wav` / `audio/mpeg` / `audio/mp4` / `application/octet-stream` |
| `Content-Length` | 文件大小 |
| `Content-Disposition` | `attachment; filename="<filename>"` |
| `Last-Modified` | 文件修改时间 |
| `ETag` | 文件标识（由 `http.ServeContent` 生成） |
| `Accept-Ranges` | `bytes`（表示支持 Range 请求） |

**失败响应**

| 状态码 | 含义 |
|--------|------|
| 400 Bad Request | 文件 ID 包含 `..` / `/` / `\` 等非法字符 |
| 404 Not Found | 文件不存在 |

**请求示例**

```bash
# 完整下载
curl -O http://localhost:8000/api/files/video123.mp4/download

# 断点续传
curl -H "Range: bytes=0-1023" \
  -o partial.bin \
  http://localhost:8000/api/files/video123.mp4/download
```

---

## 4. 删除文件（硬删除）

**接口描述**：直接删除文件，**不保留软删除记录**（v1.4 的 `deleted_files.json` 已移除）。

**请求**

```http
DELETE /api/files/{id}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string | 是 | 文件 ID（= 上传时的客户端文件名，含扩展名） |

**成功响应** (200)

```json
{
  "success": true,
  "message": "File deleted successfully"
}
```

**失败响应**

| 状态码 | 含义 |
|--------|------|
| 400 Bad Request | 文件 ID 包含非法字符 |
| 404 Not Found | 文件不存在 |
| 500 Internal Server Error | 删除失败（权限/IO 错误） |

```json
{
  "success": false,
  "error": "错误描述"
}
```

**请求示例**

```bash
curl -X DELETE http://localhost:8000/api/files/video123.mp4
```

---

## 5. 查询文件列表

**接口描述**：列出存储目录中的文件，支持按前缀/后缀过滤。

**请求**

```http
POST /api/files/query
Content-Type: application/json
```

**请求体**

```json
{
  "filters": {
    "prefix": "douyin_",
    "suffix": ".mp4"
  }
}
```

**请求字段**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| filters | object | 否 | 过滤条件；省略则返回所有文件 |
| filters.prefix | string | 否 | 文件名必须以此前缀开头 |
| filters.suffix | string | 否 | 文件名必须以此后缀结尾（如 `.mp4`） |

**成功响应** (200)

```json
{
  "success": true,
  "videos": [
    {
      "id": "douyin_video1",
      "filename": "douyin_video1.mp4",
      "size": 102400000,
      "url": "/audio/douyin_video1.mp4"
    }
  ]
}
```

**响应字段**

| 字段 | 类型 | 说明 |
|------|------|------|
| success | boolean | 查询是否成功 |
| videos | array | 文件列表 |
| videos[].id | string | 文件 ID（完整文件名，含扩展名） |
| videos[].filename | string | 完整文件名 |
| videos[].size | number | 文件大小（字节） |
| videos[].url | string | 静态直连 URL（走 `/audio/` 旁路） |

**失败响应**

| 状态码 | 含义 |
|--------|------|
| 400 Bad Request | 请求体 JSON 解析失败 |
| 500 Internal Server Error | 读取目录失败 |

**请求示例**

```bash
# 列出所有文件
curl -X POST http://localhost:8000/api/files/query

# 仅 .mp4
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"suffix": ".mp4"}}' \
  http://localhost:8000/api/files/query

# 前缀 + 后缀组合
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"prefix": "douyin_", "suffix": ".mp4"}}' \
  http://localhost:8000/api/files/query
```

---

## 6. 静态文件直连（旁路）

**接口描述**：无需走 API，浏览器/客户端可直接通过静态路径访问文件。

**请求**

```http
GET /audio/{filename}
```

由 Go 标准 `http.FileServer` 实现，自动支持：
- 目录列表（如果指向目录）
- Range 请求
- Content-Type 自动识别
- ETag / Last-Modified 缓存头

**注意**：与第 3 节的 `/api/files/{id}/download` 行为基本一致，但不走应用层 handler。删除/更新文件时请注意浏览器缓存。

---

## 7. 上传文件（可选解压）— v2.1.0 新增

**接口描述**：在 §2 上传行为基础上，可选在上传压缩包后自动解压到存储目录下的同名子目录。

> **兼容性**：**未传 `extract` 时，请求/响应与 v2.0.0 完全一致**（仅返回 `success` / `filename` / `url` / `size`，不会解压，也不会返回 `extracted_dir`）。

**请求**

```http
POST /api/files?extract=true
Content-Type: multipart/form-data
```

**可选参数**

| 参数 | 位置 | 类型 | 必填 | 说明 |
|------|------|------|------|------|
| extract | query 或 form | string | 否 | 设为 `true` 时，在上传成功后尝试解压 |
| file | form | File | 是 | 同 §2 |

**支持的压缩格式**：`.zip`、`.tar`、`.tar.gz`、`.tgz`

**成功响应** (200)

与 §2 相同，且当 `extract=true` 且解压成功时额外包含：

```json
{
  "success": true,
  "filename": "pkg.zip",
  "url": "/api/files/pkg.zip/download",
  "size": 12345,
  "extracted_dir": "pkg"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| extracted_dir | string | 解压目标目录名（相对 `audio_dir` 的一级子目录名，= 压缩包去扩展名后的 basename） |

**失败响应**

上传失败时同 §2。上传成功但解压失败时仍返回 200，`success: true`，并在 `error` 字段描述解压错误（`extracted_dir` 省略）。

**请求示例**

```bash
# 上传并解压
curl -X POST \
  -F "file=@pkg.zip" \
  -F "extract=true" \
  "http://localhost:8000/api/files?extract=true"

# 普通上传（与 v2.0.0 相同，不传 extract）
curl -X POST -F "file=@video.mp4" http://localhost:8000/api/files
```

---

## 8. 目录浏览列表 — v2.1.0 新增

**接口描述**：列出存储目录（或指定子目录）下的文件和子目录，供 Web UI 浏览使用。**不改** §5 的 `POST /api/files/query` 语义。

**请求**

```http
GET /api/files/list?path=&prefix=
```

**Query 参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| path | string | 否 | 相对 `audio_dir` 的子路径（如 `pkg/sub`）；省略或空字符串表示根目录 |
| prefix | string | 否 | 条目名称前缀过滤（仅当前目录，非递归） |

**成功响应** (200)

```json
{
  "success": true,
  "path": "pkg",
  "entries": [
    {
      "name": "sub",
      "type": "dir",
      "size": 0,
      "url": ""
    },
    {
      "name": "hello.txt",
      "type": "file",
      "size": 12,
      "url": "/audio/pkg/hello.txt"
    }
  ]
}
```

**响应字段**

| 字段 | 类型 | 说明 |
|------|------|------|
| success | boolean | 是否成功 |
| path | string | 当前浏览路径（相对路径，无首尾 `/`） |
| entries | array | 当前目录条目 |
| entries[].name | string | 文件或目录名 |
| entries[].type | string | `file` 或 `dir` |
| entries[].size | number | 文件大小（字节）；目录为 0 |
| entries[].url | string | 文件静态访问 URL（`/audio/...`）；目录为空字符串 |

**失败响应**

| 状态码 | 含义 |
|--------|------|
| 400 Bad Request | `path` 含 `..` 等非法字符 |
| 404 Not Found | 路径不存在 |
| 500 Internal Server Error | 读取目录失败 |

**请求示例**

```bash
# 根目录
curl "http://localhost:8000/api/files/list"

# 子目录 + 前缀
curl "http://localhost:8000/api/files/list?path=pkg&prefix=hel"
```

---

## 9. 删除目录 — v2.1.0 新增

**接口描述**：删除 `audio_dir` 下的一级子目录及其全部内容（`os.RemoveAll`）。

**请求**

```http
DELETE /api/files/dir/{name}
```

**路径参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 一级子目录名（不含 `/`） |

**成功响应** (200)

```json
{
  "success": true,
  "message": "Directory deleted successfully"
}
```

**失败响应**

| 状态码 | 含义 |
|--------|------|
| 400 Bad Request | 名称非法 / 目标不是目录 |
| 404 Not Found | 目录不存在 |
| 500 Internal Server Error | 删除失败 |

**请求示例**

```bash
curl -X DELETE "http://localhost:8000/api/files/dir/pkg"
```

---

## 10. Web UI 登录 / 登出 — v2.1.0 新增

**接口描述**：为内嵌 Web 文件管理 UI 提供密码登录与会话 Cookie（`ui_session`）。**文件 API（`/api/files/*`）与 `/audio/*` 不要求 UI 登录**；仅 `/` 页面在未登录时重定向至 `/login.html`。

**登录**

```http
POST /api/ui/login
Content-Type: application/json
```

**请求体**

```json
{
  "password": "change-me"
}
```

密码来自 `config.yaml` 的 `ui.password`（参见 `config.yaml.example`）。

**成功响应** (200)：设置 `HttpOnly` Cookie `ui_session`，有效期由 `ui.session_days` 控制（默认 7 天）。

```json
{
  "success": true
}
```

**失败响应** (401)

```json
{
  "success": false,
  "error": "invalid credentials"
}
```

**登出**

```http
POST /api/ui/logout
```

**成功响应** (200)：清除 `ui_session` Cookie。

```json
{
  "success": true
}
```

**请求示例**

```bash
# 登录（保存 Cookie）
curl -c cookies.txt -X POST \
  -H "Content-Type: application/json" \
  -d '{"password":"change-me"}' \
  http://localhost:8000/api/ui/login

# 登出
curl -b cookies.txt -X POST http://localhost:8000/api/ui/logout
```

---

## 安全约定

| 维度 | 实现 |
|------|------|
| 路径遍历防护 | 文件 ID 校验 `..` / `/` / `\` |
| 文件大小限制 | `config.yaml` 中 `storage.max_upload_mb`（默认 100MB） |
| 文件名冲突 | 上传同名文件**直接覆盖**（无 ID 生成，由调用方保证唯一性） |
| 并发写 | 无锁，由 OS 文件系统保证原子性 |

## 错误响应通用格式

```json
{
  "success": false,
  "error": "可读错误描述"
}
```

| 状态码 | 含义 |
|--------|------|
| 400 | 客户端请求错误（参数非法、超限） |
| 404 | 资源不存在 |
| 500 | 服务端内部错误（IO、权限） |

---

## 附录

### A. 相关文档

- [README.md](../README.md)
- [技术规范文档](技术规范文档.md)
- [ECS文件服务器部署指南](ECS文件服务器部署指南.md)

### B. 客户端示例

**Go**：

```go
// 上传
resp, _ := http.Post("http://server:8000/api/files",
    "multipart/form-data",
    fileBody)

// 下载
resp, _ := http.Get("http://server:8000/api/files/video.mp4/download")
io.Copy(dst, resp.Body)
```

**Python**：

```python
import requests

# 上传
requests.post("http://server:8000/api/files",
    files={"file": open("video.mp4", "rb")})

# 下载
requests.get("http://server:8000/api/files/video.mp4/download",
    stream=True)
```

### C. 变更记录

| 日期 | 变更 |
|------|------|
| 2026-07-17 | v2.1.0：Web UI 登录、list/deleteDir、上传可选 extract；未传 extract 时与 v2.0.0 一致 |
| 2026-06-08 | v2.0.0 重大重构：剥离业务接口，路由重命名，硬删除 |
