# Go 文件服务器

基于 Go 语言开发的轻量级文件上传服务，用于在阿里云 ECS 服务器上托管音频/视频文件。

## 功能特性

- ✅ **文件上传**: 支持最大 100MB 的文件上传
- ✅ **元数据支持**: 支持标题、作者、描述等元数据存储
- ✅ **静态文件服务**: 提供已上传文件的 HTTP 访问
- ✅ **文件检查**: 检查服务器上是否已存在指定文件
- ✅ **文件删除**: 删除文件及其关联的元数据
- ✅ **元数据查询**: 获取视频的元数据信息
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

### 1. 健康检查

```bash
curl http://localhost:8000/
```

响应:

```json
{
  "service": "Audio File Server (Go)",
  "status": "running",
  "version": "1.2.0",
  "upload_endpoint": "/upload",
  "audio_dir": "./audio_files"
}
```

### 2. 文件上传（支持元数据）

```bash
curl -X POST \
  -F "file=@video.mp4" \
  -F "title=视频标题" \
  -F "author=作者名称" \
  -F "description=视频描述" \
  http://localhost:8000/upload
```

响应:

```json
{
  "success": true,
  "filename": "video.mp4",
  "url": "/audio/video.mp4",
  "size": 12345,
  "metadata": {
    "filename": "video.mp4",
    "title": "视频标题",
    "author": "作者名称",
    "description": "视频描述",
    "upload_time": "2026-02-27T12:00:00+08:00"
  }
}
```

### 3. 文件访问

```bash
curl http://localhost:8000/audio/audio.wav --output downloaded.wav
```

### 4. 文件检查

```bash
curl http://localhost:8000/api/check/video.mp4
```

文件存在时响应:
```json
{
  "exists": true,
  "filename": "video.mp4",
  "size": 102400000,
  "upload_time": "2026-02-26T12:00:00Z"
}
```

文件不存在时响应:
```json
{
  "exists": false,
  "filename": "video.mp4"
}
```

### 5. 获取视频元数据

```bash
curl http://localhost:8000/api/metadata/video.mp4
```

响应:
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

### 6. 删除视频及元数据

```bash
curl -X DELETE http://localhost:8000/api/file/video.mp4
```

响应:
```json
{
  "success": true,
  "message": "File deleted successfully"
}
```

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
