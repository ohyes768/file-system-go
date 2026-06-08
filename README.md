# Go 文件服务器

基于 Go 语言开发的轻量级文件存储服务（v2.0），用于在阿里云 ECS 服务器上托管任意类型的文件。**纯文件存储，不管理业务元数据**——标题/作者/描述等元数据由上层应用（如 douyin-processor）维护。

## 功能特性

- ✅ **文件上传**: 支持最大 100MB 的 multipart 文件上传
- ✅ **文件下载**: 支持 HTTP Range，断点续传
- ✅ **文件硬删除**: 直接删除文件，不留软删除记录
- ✅ **文件列表查询**: 支持按前缀/后缀过滤
- ✅ **静态文件服务**: 提供已上传文件的 HTTP 直连访问
- ✅ **健康检查**: 服务状态监控接口
- ✅ **双日志输出**: 控制台 + 文件日志
- ✅ **配置文件**: YAML 格式配置管理
- ✅ **单文件部署**: 编译后只需一个可执行文件
- ✅ **跨平台**: 支持 Windows/Linux 交叉编译

## 快速开始

### 前置条件

- Go 1.21+
- Git Bash (Windows) 或终端 (Linux/macOS)

### 本地开发测试

#### 1. 编译 Windows 版本

```bash
bash scripts/build-test.sh
```

编译成功后会在 `bin/` 目录生成 `audio-server.exe`

#### 2. 启动服务器

```bash
cd bin
./audio-server.exe
```

或使用启动脚本:

```bash
cd bin
./start-server.sh
```

#### 3. 运行测试

在新的终端窗口运行:

```bash
bash scripts/test-api.sh
```

### 服务器部署

#### 1. 编译 Linux 版本

```bash
bash scripts/build.sh
```

编译成功后会在 `bin/` 目录生成 `audio-server` (Linux 可执行文件)

#### 2. 上传到服务器

```bash
scp bin/audio-server root@your-ecs-ip:/root/
```

#### 3. 服务器配置

登录服务器并配置:

```bash
ssh root@your-ecs-ip

# 创建音频目录
mkdir -p /var/www/audio
chmod 755 /var/www/audio

# 设置可执行权限
chmod +x /root/audio-server

# 测试运行
./audio-server
```

#### 4. 配置 systemd 服务

创建 `/etc/systemd/system/audio-file-server.service`:

```ini
[Unit]
Description=Audio File Server (Go)
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root
ExecStart=/root/audio-server
Restart=always
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

启动服务:

```bash
systemctl daemon-reload
systemctl start audio-file-server
systemctl enable audio-file-server
systemctl status audio-file-server
```

## 项目结构

```
file-system-go/
├── main.go              # 主程序入口
├── config.yaml          # 配置文件
├── go.mod               # Go 模块依赖
├── go.sum               # 依赖版本锁定
├── scripts/             # 脚本目录
│   ├── build.sh         # Linux 编译脚本
│   ├── build-test.sh    # Windows 本地编译脚本
│   └── test-api.sh      # API 测试脚本
├── logs/                # 日志目录
├── docs/                # 文档目录
├── bin/                 # 编译输出目录
└── test/                # 测试文件目录
```

## 配置说明

`config.yaml` 配置文件:

```yaml
server:
  port: "8000"              # 监听端口
  read_timeout: 300         # 读取超时（秒）
  write_timeout: 300        # 写入超时（秒）

storage:
  audio_dir: "./audio_files"  # 音频文件存储目录
  max_upload_mb: 100          # 最大上传文件大小（MB）

logging:
  level: "INFO"              # 日志级别
  log_dir: "./logs"          # 日志目录
```

## API 接口

> **基础路径**：`/api/files`，文件 ID = 上传时的客户端文件名（含扩展名）。
> 本服务只做文件存取，**业务元数据（标题/作者/描述）由上层应用维护**。

### 1. 健康检查

```bash
curl http://localhost:8000/health
```

响应:

```json
{
  "service": "File Server (Go)",
  "status": "running",
  "version": "2.0.0",
  "audio_dir": "./audio_files"
}
```

### 2. 上传文件

```bash
curl -X POST \
  -F "file=@video.mp4" \
  http://localhost:8000/api/files
```

响应:

```json
{
  "success": true,
  "filename": "video.mp4",
  "url": "/api/files/video.mp4/download",
  "size": 12345
}
```

### 3. 下载文件

```bash
# 完整下载
curl -O http://localhost:8000/api/files/video.mp4/download

# 断点续传（HTTP Range）
curl -H "Range: bytes=0-1023" \
  -o partial.bin \
  http://localhost:8000/api/files/video.mp4/download
```

### 4. 删除文件（硬删除）

```bash
curl -X DELETE http://localhost:8000/api/files/video.mp4
```

响应:
```json
{
  "success": true,
  "message": "File deleted successfully"
}
```

### 5. 查询文件列表

```bash
# 查询所有文件
curl -X POST http://localhost:8000/api/files/query

# 按后缀过滤（仅 .mp4）
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"suffix": ".mp4"}}' \
  http://localhost:8000/api/files/query

# 按前缀过滤
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"prefix": "douyin_"}}' \
  http://localhost:8000/api/files/query

# 组合过滤
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"filters":{"prefix": "douyin_", "suffix": ".mp4"}}' \
  http://localhost:8000/api/files/query
```

响应:
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

### 6. 静态文件直连（旁路）

无需走 API，浏览器/客户端可直接通过静态路径访问：

```bash
curl http://localhost:8000/audio/video.mp4 --output downloaded.mp4
```

## 接口对照表

| 方法 | 路径 | 作用 | v1 → v2 变更 |
|---|---|---|---|
| `GET` | `/health` | 健康检查 | `/` → `/health` |
| `POST` | `/api/files` | 上传文件 | `/upload` → `/api/files`（移除 metadata 字段） |
| `GET` | `/api/files/{id}/download` | 下载文件 | `/api/videos/{id}/download` 路径迁移 |
| `DELETE` | `/api/files/{id}` | 删除文件 | `/api/file/{filename}` + `/api/videos/{filename}` → 统一 |
| `POST` | `/api/files/query` | 文件列表 | `/api/videos/query` 路径迁移 |
| `GET` | `/audio/{filename}` | 静态直连 | 保留 |

**已移除接口**（v1.4 / v1.5 业务相关，已迁出至 douyin-processor）：

- `GET /api/check/{filename}` — 文件检查
- `GET /api/metadata/{filename}` — 元数据查询
- `GET/POST/DELETE /api/deleted/*` — 已删除文件管理
- `GET/POST/DELETE /api/read/*` — 已读文件管理
- `GET/DELETE /api/uncollected/*` — 取消收藏管理

## 日志

日志文件位置: `logs/audio-server-{date}.log`

查看实时日志:

```bash
tail -f logs/audio-server-$(date +%Y-%m-%d).log
```

服务器环境查看 systemd 日志:

```bash
journalctl -u audio-file-server -f
```

## 技术栈

- **Go 1.21+**: 高性能编程语言
- **Gorilla Mux**: HTTP 路由器
- **YAML**: 配置文件格式

## 文档

- [技术规范文档](docs/技术规范文档.md)
- [ECS文件服务器部署指南](ECS文件服务器部署指南.md)

## 开发说明

### 代码规范

- 主程序不超过 400 行
- 使用 YAML 配置文件
- 日志输出到控制台和文件
- 支持交叉编译

### 添加依赖

```bash
go get github.com/package/name
go mod tidy
```

## 常见问题

### 1. 端口被占用

修改 `config.yaml` 中的 `port` 配置

### 2. 文件上传失败

检查:
- 文件大小是否超过限制
- 音频目录是否存在且有写权限
- 磁盘空间是否充足

### 3. 日志文件过大

定期清理旧日志或配置日志轮转策略

## License

MIT
