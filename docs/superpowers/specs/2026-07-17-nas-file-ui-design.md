# NAS 简易文件站设计

**日期：** 2026-07-17  
**状态：** 已批准  
**项目：** file-system-go

## 1. 目标

在现有 Go 文件服务上增加简单 Web 管理界面，支持上传不超过约 1GB 的 zip/tar 包并自动解压，供 NAS 上的 agent 通过内网 HTTP 拉取文件，避免从 GitHub 下载大包过慢。

## 2. 约束与决策

| 项 | 决策 |
|----|------|
| 单文件大小 | ≤ 1GB，流式直传即可，不做分片 |
| 解压 | 上传后服务端自动解压 |
| 鉴权 | 仅 Web 界面要密码；`/api/files*` 与静态下载对内网 agent 免密 |
| 前端范围 | 够用：上传进度、前缀搜索、一键复制下载 URL、列表/目录浏览、删除 |
| 实现形态 | Go 内嵌静态页（`embed`），保持单二进制 + 现有 Docker 部署 |
| 原压缩包 | 解压后保留 |
| 解压目录 | 以包名命名的子目录，如 `foo.zip` → `foo/` |
| 接口兼容 | **不破坏现有对外 API**（见 §2.1） |

### 2.1 接口兼容硬约束

现有调用方（如 `douyin-processor`）依赖的接口必须保持可用，规则如下：

| 接口 | 兼容要求 |
|------|----------|
| `GET /health` | 路径、方法、现有响应字段不变（可附加字段） |
| `POST /api/files` | 仍为 `multipart` 字段名 `file`；成功响应至少含 `success` / `filename` / `url` / `size`；**默认不解压** |
| `GET /api/files/{id}/download` | 行为不变（含 Range） |
| `DELETE /api/files/{id}` | 对**文件**的硬删除行为不变；目录删除走新能力，不改变原「删文件」语义 |
| `POST /api/files/query` | 请求体与响应结构不变 |
| `GET /audio/{filename}` | 静态直连不变 |

新增能力一律走**新路径**或**显式可选参数**，例如：

- 自动解压：仅当请求带 `extract=true`（表单或 query）时执行；Web UI 上传时带上；旧客户端不传则与现在完全一致。
- 子目录浏览 / 前缀列表增强：新增如 `GET /api/files/list`（或等价新路径），**不改** `POST /api/files/query`。
- UI 登录：仅 `/api/ui/*` 与页面路由；**绝不**给 `/api/files*`、`/audio/*`、`/health` 加鉴权。

## 3. 架构

```
浏览器 ──(界面密码 Cookie)──► 内嵌 Web UI
                                  │
                                  ▼
Agent（免密 curl/wget）──────► Go HTTP API ──► 本地磁盘
                                  │
                                  └── 上传后自动解压
```

- 继续单进程服务，默认端口 `8000`，经 Nginx HTTPS 对外（现有 NAS 部署不变）。
- UI 路由与文件 API 分离：UI 中间件只保护页面与 `/api/ui/*`，不拦截文件读写 API。

## 4. 存储约定

- 根目录：配置项 `storage.audio_dir`（默认 `./audio_files`），本期不强制改名。
- 压缩包：`.zip`、`.tar`、`.tar.gz` / `.tgz` 上传后保留原文件，并解压到同名去掉扩展名的子目录。
- 非压缩文件：只存储，不解压。
- 安全：解压时拒绝路径穿越（`../` 等）；已存在目标文件则覆盖。
- 静态访问：继续使用现有 `/audio/` 前缀直链，支持 HTTP Range（本期不改名，避免破坏已有调用方）。

## 5. API

### 5.1 现有接口（行为冻结，仅允许内部优化）

- `GET /health`
- `POST /api/files` — 内部可改为流式写入、提高默认 `max_upload_mb`；**响应必填字段与默认语义不变**；不解压 unless `extract=true`
- `GET /api/files/{id}/download`（Range）
- `DELETE /api/files/{id}` — 仅删除同名**文件**（与现网一致）
- `POST /api/files/query` — 不变
- `GET /audio/{filename}` — 不变

### 5.2 新增接口（给 UI / 新能力）

- `GET /api/files/list?prefix=&path=` — 子目录浏览 + 前缀搜索（UI 使用）
- `DELETE /api/files/dir/{name}` — 删除解压目录（可选；避免改动原 DELETE 语义）
- `POST /api/files` + `extract=true` — 上传成功后自动解压；响应**可附加** `extracted_dir`（旧字段仍保留）
- `POST /api/ui/login` / `POST /api/ui/logout`
- `GET /`、`GET /login` 等 — 内嵌静态页（`/` 在 v2 已让给 `/health`，可安全用作管理首页）

## 6. Web UI

1. **登录页**：校验 `ui.password`；成功后 Cookie，有效期由 `ui.session_days`（默认 7）控制。
2. **主页**：拖拽/选择上传 + 进度条。
3. **列表区**：展示文件与解压目录；前缀搜索；下载 / 一键复制 URL / 删除。
4. **目录浏览**：进入解压目录查看内容，可复制单文件下载 URL 给 agent。

密码仅用于浏览器会话；agent 不需要 Cookie 或 API Key。

## 7. 配置

```yaml
server:
  port: "8000"
  read_timeout: 3600
  write_timeout: 3600
storage:
  audio_dir: "./audio_files"
  max_upload_mb: 1024
ui:
  password: "change-me"
  session_days: 7
logging:
  level: "INFO"
  log_dir: "./logs"
```

- 必须把 `read_timeout` / `write_timeout` 应用到 `http.Server`（当前配置存在但未生效）。
- 仓库提供 `config.yaml.example`；真实 `config.yaml` 仍不入库。

## 8. 明确不做（本期）

- API 鉴权 / API Key
- 分片上传 / 断点续传上传
- GitHub 自动镜像同步
- 多用户、数据库、业务元数据
- 独立前端工程（Vue/React 等）

## 9. 验收标准

1. 浏览器输入密码后可上传约 1GB 的 zip，看到上传进度，并自动出现解压目录。
2. 可按前缀搜索，可一键复制下载 URL。
3. NAS 上 agent 无需密码即可 `curl`/`wget` 下载压缩包或解压后的文件。
4. 现有 Docker Compose + Nginx 部署路径可平滑升级（替换二进制/挂载静态资源即可）。
5. **回归**：不带 `extract` 的 `POST /api/files`、`POST /api/files/query`、download、delete、`/audio/`、`/health` 与升级前行为一致（可用现有 `scripts/test-api.sh` 验证）。

## 10. 实现提示

- 核心逻辑仍可从单文件 `main.go` 拆出：`upload`（流式）、`extract`（zip/tar）、`ui`（session + 静态）、`handlers`。
- 上传进度由浏览器 `XMLHttpRequest`/`fetch` + `upload.onprogress` 完成，服务端无需 SSE。
- Session 可用签名 Cookie（HMAC + 过期时间）或服务端内存 session；单实例 NAS 场景任选其一，优先简单签名 Cookie，避免额外存储。
