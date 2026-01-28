# ECS 文件服务器部署指南 (Go 版本)

本指南说明如何在阿里云 ECS 服务器上部署基于 Go 语言开发的文件上传服务，用于托管音频文件。

## 为什么选择 Go？

- **单文件部署**：编译后只需一个可执行文件，无需依赖
- **高性能**：并发性能优异，适合文件传输场景
- **跨平台**：一次编译，多平台运行
- **低资源占用**：内存和 CPU 占用远低于 Python 方案

## 前提条件

- 已有阿里云 ECS 服务器
- ECS 服务器具有公网 IP
- ECS 安全组已开放 8000 端口
- 本地开发环境需要安装 Go 1.21+（用于编译）

## 部署步骤

### 方案一：本地编译，服务器部署（推荐）

这种方式适合快速部署，只需在服务器上运行一个编译好的二进制文件。

#### 步骤 1：本地准备 Go 项目

首先在本地创建 Go 项目代码。

**创建项目目录：**

```bash
# 在本地创建项目目录
mkdir -p ~/go-audio-server
cd ~/go-audio-server
```

**初始化 Go 模块：**

```bash
go mod init audio-server
```

**创建主程序文件 `main.go`：**

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
)

// 配置常量
const (
	AUDIO_DIR     = "/var/www/audio"
	PORT          = "8000"
	MAX_UPLOAD_MB = 100 // 最大上传文件大小 100MB
)

// 响应结构体
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type UploadResponse struct {
	Success bool   `json:"success"`
	Filename string `json:"filename,omitempty"`
	URL      string `json:"url,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Error    string `json:"error,omitempty"`
}

type HealthResponse struct {
	Service       string `json:"service"`
	Status        string `json:"status"`
	UploadEndpoint string `json:"upload_endpoint"`
	AudioDir      string `json:"audio_dir"`
}

func init() {
	// 确保音频目录存在
	if err := os.MkdirAll(AUDIO_DIR, 0755); err != nil {
		log.Fatalf("无法创建音频目录: %v", err)
	}
}

// 健康检查接口
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HealthResponse{
		Service:        "Audio File Server (Go)",
		Status:         "running",
		UploadEndpoint: "/upload",
		AudioDir:       AUDIO_DIR,
	}
	json.NewEncoder(w).Encode(response)
}

// 文件上传接口
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 限制上传大小（100MB）
	r.Body = http.MaxBytesReader(w, r.Body, MAX_UPLOAD_MB*1024*1024)

	// 解析表单
	if err := r.ParseMultipartForm(MAX_UPLOAD_MB << 20); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "文件过大或解析失败",
		})
		return
	}

	// 获取上传的文件
	file, header, err := r.FormFile("file")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   "未找到上传文件",
		})
		return
	}
	defer file.Close()

	// 构建文件保存路径
	filename := header.Filename
	filePath := filepath.Join(AUDIO_DIR, filename)

	// 创建目标文件
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

	// 复制文件内容
	size, err := io.Copy(dst, file)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(UploadResponse{
			Success: false,
			Error:   fmt.Sprintf("文件保存失败: %v", err),
		})
		return
	}

	// 构建访问 URL
	fileURL := fmt.Sprintf("/audio/%s", filename)

	// 返回成功响应
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(UploadResponse{
		Success: true,
		Filename: filename,
		URL:      fileURL,
		Size:     size,
	})
}

// 日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)
		next.ServeHTTP(w, r)
		log.Printf("完成耗时: %v", time.Since(start))
	})
}

func main() {
	r := mux.NewRouter()

	// 注册路由
	r.HandleFunc("/", healthHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("POST")

	// 静态文件服务
	r.PathPrefix("/audio/").Handler(http.StripPrefix("/audio/", http.FileServer(http.Dir(AUDIO_DIR))))

	// 应用中间件
	handler := loggingMiddleware(r)

	// 启动服务器
	addr := ":" + PORT
	log.Printf("🚀 音频文件服务器启动成功!")
	log.Printf("📁 音频目录: %s", AUDIO_DIR)
	log.Printf("🌐 监听地址: 0.0.0.0:%s", PORT)
	log.Printf("✅ 健康检查: http://localhost:%s/", PORT)
	log.Printf("📤 上传接口: http://localhost:%s/upload", PORT)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
```

**安装依赖：**

```bash
go get github.com/gorilla/mux
```

#### 步骤 2：交叉编译

在本地编译 Linux 版本的可执行文件。

**编译 Linux AMD64 版本（适用于大多数云服务器）：**

```bash
# 设置编译目标
export GOOS=linux
export GOARCH=amd64

# 编译（会生成 audio-server 可执行文件）
go build -o audio-server main.go

# 验证编译结果
ls -lh audio-server
file audio-server
```

**编译 Linux ARM64 版本（适用于 ARM 架构的服务器）：**

```bash
export GOOS=linux
export GOARCH=arm64
go build -o audio-server-arm64 main.go
```

#### 步骤 3：上传到服务器

**使用 SCP 上传：**

```bash
# 上传可执行文件到服务器
scp audio-server root@your-ecs-ip:/root/

# 上传到服务器后登录
ssh root@your-ecs-ip
```

#### 步骤 4：服务器配置

**1. 创建音频文件目录：**

```bash
# 创建目录
mkdir -p /var/www/audio

# 设置权限
chmod 755 /var/www/audio
```

**2. 设置可执行权限：**

```bash
cd /root
chmod +x audio-server
```

**3. 测试运行（前台运行）：**

```bash
./audio-server
```

看到以下输出表示启动成功：

```
2025/01/28 10:00:00 🚀 音频文件服务器启动成功!
2025/01/28 10:00:00 📁 音频目录: /var/www/audio
2025/01/28 10:00:00 🌐 监听地址: 0.0.0.0:8000
2025/01/28 10:00:00 ✅ 健康检查: http://localhost:8000/
2025/01/28 10:00:00 📤 上传接口: http://localhost:8000/upload
```

**4. 测试健康检查：**

打开新终端，测试服务是否正常：

```bash
curl http://localhost:8000/
```

应返回：

```json
{
  "service": "Audio File Server (Go)",
  "status": "running",
  "upload_endpoint": "/upload",
  "audio_dir": "/var/www/audio"
}
```

按 `Ctrl+C` 停止测试运行。

#### 步骤 5：配置 systemd 服务（后台运行）

**创建 systemd 服务文件：**

```bash
# 创建 systemd 服务文件
cat > /etc/systemd/system/audio-file-server.service << 'EOF'
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

# 日志配置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=audio-server

# 安全加固
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF
```

**启动和管理服务：**

```bash
# 重载 systemd 配置
systemctl daemon-reload

# 启动服务
systemctl start audio-file-server

# 设置开机自启
systemctl enable audio-file-server

# 查看服务状态
systemctl status audio-file-server

# 查看实时日志
journalctl -u audio-file-server -f

# 查看最近 100 条日志
journalctl -u audio-file-server -n 100
```

服务管理常用命令：

```bash
# 停止服务
systemctl stop audio-file-server

# 重启服务
systemctl restart audio-file-server

# 查看服务是否开机自启
systemctl is-enabled audio-file-server

# 禁用开机自启
systemctl disable audio-file-server
```

### 方案二：服务器直接编译（可选）

如果你想在服务器上直接编译 Go 代码，可以使用这种方式。

#### 步骤 1：在服务器上安装 Go

**CentOS/Alibaba Cloud Linux：**

```bash
# 下载 Go 1.21.6
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz

# 解压到 /usr/local
tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz

# 配置环境变量
cat >> /etc/profile << 'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=/root/go
export PATH=$PATH:$GOPATH/bin

# 配置国内 Go 代理（加速依赖下载）
export GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
export GO111MODULE=on
EOF

# 使环境变量生效
source /etc/profile

# 验证安装
go version

# 验证代理配置
go env GOPROXY
```

**Ubuntu/Debian：**

```bash
# 安装 Go
apt update
apt install golang-go -y

# 验证安装
go version
```

#### 步骤 2：创建项目并编译

```bash
# 创建项目目录
mkdir -p /root/go-audio-server
cd /root/go-audio-server

# 初始化模块
go mod init audio-server

# 安装依赖
go get github.com/gorilla/mux

# 创建 main.go（使用方案一中的代码）
# ... 将 main.go 内容写入 ...

# 编译
go build -o audio-server main.go

# 设置执行权限
chmod +x audio-server
```

后面的步骤与方案一相同（创建目录、配置 systemd 等）。

### 配置防火墙（如果有）

```bash
# CentOS/Alibaba Cloud Linux
firewall-cmd --permanent --add-port=8000/tcp
firewall-cmd --reload

# Ubuntu/Debian (使用 ufw)
ufw allow 8000/tcp
```

### 配置安全组

在阿里云控制台：
1. 进入 ECS 实例列表
2. 点击实例ID进入详情页
3. 点击"安全组"标签
4. 配置规则 -> 添加安全组规则：
   - 端口范围：8000/8000
   - 授权对象：0.0.0.0/0

### 更新本地配置文件

在本地项目中的 `config/asr_config.yaml` 文件中，更新 ECS 地址：

```yaml
ecs:
  # 替换为你的 ECS 公网 IP
  host: "http://your-ecs-ip:8000"
  upload_endpoint: "/upload"
  file_dir: "/var/www/audio"
```

## 测试

### 测试健康检查

```bash
# 本地测试
curl http://your-ecs-ip:8000/
```

应返回：

```json
{
  "service": "Audio File Server (Go)",
  "status": "running",
  "upload_endpoint": "/upload",
  "audio_dir": "/var/www/audio"
}
```

### 测试文件上传

```bash
# 在本地测试上传接口
curl -X POST \
  -F "file=@test_audio.wav" \
  http://your-ecs-ip:8000/upload
```

应返回：

```json
{
  "success": true,
  "filename": "test_audio.wav",
  "url": "/audio/test_audio.wav",
  "size": 12345
}
```

### 测试文件访问

```bash
# 访问上传的文件
curl http://your-ecs-ip:8000/audio/test_audio.wav --output downloaded.wav
```

## 常见问题

### 1. 无法访问服务

**检查清单：**

```bash
# 1. 检查服务是否运行
systemctl status audio-file-server

# 2. 检查端口是否监听
netstat -tlnp | grep 8000
# 或
ss -tlnp | grep 8000

# 3. 检查防火墙
firewall-cmd --list-ports  # CentOS
ufw status                 # Ubuntu

# 4. 检查安全组（在阿里云控制台）
```

**解决方案：**

- 确认安全组已开放 8000 端口
- 确认防火墙允许 8000 端口
- 确认服务正在运行

### 2. 文件上传失败

**可能原因和解决方案：**

```bash
# 检查目录权限
ls -la /var/www/ | grep audio

# 修复权限
chmod 755 /var/www/audio
chown root:root /var/www/audio

# 检查磁盘空间
df -h

# 查看服务日志
journalctl -u audio-file-server -n 50
```

### 3. 服务启动失败

**查看详细错误信息：**

```bash
# 查看服务状态
systemctl status audio-file-server

# 查看详细日志
journalctl -u audio-file-server -n 100 --no-pager

# 手动运行测试
cd /root
./audio-server
```

**常见问题：**

- 文件不存在：确认 `/root/audio-server` 文件存在
- 权限不足：`chmod +x /root/audio-server`
- 端口占用：`netstat -tlnp | grep 8000` 查看占用进程
- Go 版本不兼容：确保使用 Go 1.21+ 编译

### 4. 权限问题

```bash
# 修改目录所有者
chown -R root:root /var/www/audio

# 设置权限
chmod 755 /var/www/audio
```

### 5. Go 依赖下载超时（国内环境）

**问题表现：**
```
go: downloading github.com/gorilla/mux v1.8.1
go: downloading gopkg.in/yaml.v3 v3.0.1
main.go:13:2: github.com/gorilla/mux@v1.8.1: Get "https://proxy.golang.org/...": dial tcp: i/o timeout
```

**解决方案：**

```bash
# 临时配置 Go 代理
export GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
export GO111MODULE=on

# 验证配置
go env GOPROXY

# 重新下载依赖
go mod download

# 永久配置（添加到 ~/.bashrc 或 /etc/profile）
cat >> ~/.bashrc << 'EOF'
export GOPROXY=https://goproxy.cn,direct
export GO111MODULE=on
EOF
source ~/.bashrc
```

**注意：** 编译脚本 `scripts/build.sh` 和 `scripts/build-test.sh` 已自动配置 GOPROXY，无需手动配置。

## 生产环境建议

### 使用 Nginx 反向代理

**安装 Nginx：**

```bash
# CentOS/Alibaba Cloud Linux
yum install nginx -y

# Ubuntu/Debian
apt install nginx -y
```

**配置 Nginx：**

```bash
# 创建配置文件
cat > /etc/nginx/conf.d/audio-server.conf << 'EOF'
server {
    listen 80;
    server_name your-domain.com;  # 替换为你的域名

    # 请求体大小限制（100MB）
    client_max_body_size 100M;

    # API 接口反向代理
    location / {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 超时配置
        proxy_connect_timeout 300s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    # 静态文件直接由 Nginx 提供（性能更好）
    location /audio/ {
        alias /var/www/audio/;
        autoindex off;
        expires 30d;  # 缓存 30 天
        add_header Cache-Control "public, immutable";
    }

    # Gzip 压缩
    gzip on;
    gzip_types text/plain application/json;
}
EOF

# 测试配置
nginx -t

# 启动 Nginx
systemctl start nginx
systemctl enable nginx
```

### 配置 HTTPS

使用 Let's Encrypt 配置免费证书：

```bash
# 安装 certbot
# CentOS/Alibaba Cloud Linux
yum install certbot python3-certbot-nginx -y

# Ubuntu/Debian
apt install certbot python3-certbot-nginx -y

# 获取证书（自动配置 Nginx）
certbot --nginx -d your-domain.com

# 测试自动续期
certbot renew --dry-run
```

### 性能优化

**1. 启用 Go 运行时优化（可选）**

在编译时添加优化参数：

```bash
# 本地编译优化版本
export GOOS=linux
export GOARCH=amd64
go build -ldflags="-s -w" -o audio-server main.go
```

参数说明：
- `-s`：去除符号表
- `-w`：去除 DWARF 调试信息
- 可执行文件体积会减小约 30-40%

**2. 调整文件描述符限制**

```bash
# 修改系统限制
cat >> /etc/security/limits.conf << 'EOF'
root soft nofile 65535
root hard nofile 65535
EOF

# 在 systemd 服务中添加
cat >> /etc/systemd/system/audio-file-server.service << 'EOF'
[Service]
LimitNOFILE=65535
EOF

systemctl daemon-reload
systemctl restart audio-file-server
```

**3. 启用 HTTP/2（Nginx）**

修改 Nginx 配置：

```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    # 其他配置...
}
```

## 监控和日志

### 查看服务日志

```bash
# 实时查看日志
journalctl -u audio-file-server -f

# 查看最近 100 行
journalctl -u audio-file-server -n 100

# 查看今天的日志
journalctl -u audio-file-server --since today

# 查看特定时间段的日志
journalctl -u audio-file-server --since "2025-01-28 10:00:00" --until "2025-01-28 12:00:00"
```

### 监控磁盘使用

```bash
# 查看目录大小
du -sh /var/www/audio

# 查看文件数量
find /var/www/audio -type f | wc -l

# 查看磁盘空间
df -h

# 按时间排序显示最近修改的文件
ls -lt /var/www/audio | head -20
```

### 监控服务性能

```bash
# 查看进程资源占用
top -p $(pidof audio-server)

# 查看内存使用
ps aux | grep audio-server

# 查看网络连接
netstat -anp | grep 8000
```

## 清理旧文件

### 方式一：手动清理

```bash
# 查找 7 天前的文件
find /var/www/audio -type f -mtime +7

# 删除 7 天前的文件
find /var/www/audio -type f -mtime +7 -delete

# 查找大于 100MB 的文件
find /var/www/audio -type f -size +100M
```

### 方式二：定时任务自动清理

```bash
# 编辑 crontab
crontab -e

# 添加每天凌晨 3 点清理 7 天前的文件
0 3 * * * find /var/www/audio -type f -mtime +7 -delete

# 添加每周日凌晨 4 点清理大于 100MB 的文件
0 4 * * 0 find /var/www/audio -type f -size +100M -delete
```

### 方式三：创建清理脚本（推荐）

```bash
# 创建清理脚本
cat > /root/cleanup-audio.sh << 'EOF'
#!/bin/bash

# 配置
AUDIO_DIR="/var/www/audio"
DAYS_TO_KEEP=7
LOG_FILE="/var/log/audio-cleanup.log"

# 记录日志
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

log "开始清理音频文件"

# 统计清理前的文件数
BEFORE_COUNT=$(find "$AUDIO_DIR" -type f | wc -l)
log "清理前文件数: $BEFORE_COUNT"

# 删除旧文件
DELETED_COUNT=$(find "$AUDIO_DIR" -type f -mtime +$DAYS_TO_KEEP -delete -print | wc -l)
log "已删除 $DELETED_COUNT 个超过 $DAYS_TO_KEEP 天的文件"

# 统计清理后的文件数和磁盘使用
AFTER_COUNT=$(find "$AUDIO_DIR" -type f | wc -l)
DISK_USAGE=$(du -sh "$AUDIO_DIR" | cut -f1)

log "清理后文件数: $AFTER_COUNT"
log "磁盘使用: $DISK_USAGE"
log "清理完成"
EOF

# 设置执行权限
chmod +x /root/cleanup-audio.sh

# 手动测试
/root/cleanup-audio.sh

# 查看日志
cat /var/log/audio-cleanup.log

# 添加到 crontab（每天凌晨 3 点执行）
crontab -e
# 添加以下行
0 3 * * * /root/cleanup-audio.sh
```

## 部署检查清单

部署完成后，请确认以下检查项：

- [ ] 服务已启动并开机自启
- [ ] 防火墙已开放 8000 端口
- [ ] 安全组已配置允许访问
- [ ] 健康检查接口正常返回
- [ ] 文件上传功能正常
- [ ] 文件访问功能正常
- [ ] systemd 日志正常输出
- [ ] 磁盘空间充足
- [ ] 已配置定时清理任务（如需要）
- [ ] 已配置 Nginx 反向代理（生产环境）
- [ ] 已配置 HTTPS（生产环境）

## 进阶：自定义配置

如果你想修改服务器配置，可以编辑 `main.go` 中的常量：

```go
const (
    AUDIO_DIR     = "/var/www/audio"  // 修改音频存储目录
    PORT          = "8000"            // 修改监听端口
    MAX_UPLOAD_MB = 100               // 修改最大上传文件大小（MB）
)
```

修改后需要重新编译：

```bash
# 本地重新编译
export GOOS=linux
export GOARCH=amd64
go build -o audio-server main.go

# 上传到服务器
scp audio-server root@your-ecs-ip:/root/

# 重启服务
ssh root@your-ecs-ip
systemctl restart audio-file-server
```

## 故障排查命令速查

```bash
# 服务状态
systemctl status audio-file-server

# 服务日志
journalctl -u audio-file-server -f

# 端口监听
ss -tlnp | grep 8000

# 磁盘空间
df -h

# 目录大小
du -sh /var/www/audio

# 进程信息
ps aux | grep audio-server

# 网络连接
netstat -anp | grep 8000

# 测试上传
curl -X POST -F "file=@test.wav" http://localhost:8000/upload

# 测试健康检查
curl http://localhost:8000/

# 防火墙规则
firewall-cmd --list-all
```
