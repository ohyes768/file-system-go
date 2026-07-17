# NAS 简易文件站 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不破坏现有 API 的前提下，为 file-system-go 增加带密码的 Web 管理页、可选自动解压的大文件上传（≤1GB）、目录浏览与复制下载 URL。

**Architecture:** 将 `main.go` 拆成 config / extract / ui-auth / handlers / main；静态页用 `embed` 打进二进制。现有 `/api/files*`、`/audio/`、`/health` 保持兼容；解压仅在 `extract=true` 时触发；UI 用 HMAC 签名 Cookie，不拦截文件 API。

**Tech Stack:** Go 1.24、gorilla/mux、标准库 `archive/zip` + `archive/tar` + `compress/gzip`、`embed`、原生 HTML/CSS/JS（无前端构建）。

**Spec:** [docs/superpowers/specs/2026-07-17-nas-file-ui-design.md](../specs/2026-07-17-nas-file-ui-design.md)

---

## File Structure

| 路径 | 职责 |
|------|------|
| `config.go` | `Config`、`loadConfig`、`setDefaultConfig` |
| `extract.go` | zip/tar/tar.gz 安全解压到目标目录 |
| `extract_test.go` | 解压与路径穿越测试 |
| `ui_auth.go` | 登录 Cookie 签发/校验、login/logout handler |
| `ui_auth_test.go` | Cookie 校验测试 |
| `handlers.go` | 现有 health/upload/download/delete/query + 新 list/deleteDir |
| `handlers_compat_test.go` | 旧 API 回归（响应字段） |
| `web/index.html` / `web/login.html` / `web/app.js` / `web/style.css` | 管理 UI |
| `web/embed.go` | `//go:embed` + FileServer |
| `main.go` | 路由注册、中间件、`http.Server` 超时 |
| `config.yaml.example` | 示例配置 |
| `docs/API接口文档.md` | 仅追加新接口说明，不改旧接口语义 |

---

### Task 1: 抽出配置并提高默认上传上限

**Files:**
- Create: `config.go`
- Modify: `main.go`（删除已迁出的 Config/load/setDefault）
- Create: `config.yaml.example`

- [ ] **Step 1: 创建 `config.go`**

把现有 `Config`、`loadConfig`、`setDefaultConfig` 移到 `config.go`，并增加 `UI` 段；默认超时与上传上限按设计调整：

```go
package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port         string `yaml:"port"`
		ReadTimeout  int    `yaml:"read_timeout"`
		WriteTimeout int    `yaml:"write_timeout"`
	} `yaml:"server"`
	Storage struct {
		AudioDir    string `yaml:"audio_dir"`
		MaxUploadMB int    `yaml:"max_upload_mb"`
	} `yaml:"storage"`
	UI struct {
		Password    string `yaml:"password"`
		SessionDays int    `yaml:"session_days"`
	} `yaml:"ui"`
	Logging struct {
		Level  string `yaml:"level"`
		LogDir string `yaml:"log_dir"`
	} `yaml:"logging"`
}

var config Config

func loadConfig() error {
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		consoleLogger.Printf("警告: 无法读取配置文件，使用默认配置: %v", err)
		setDefaultConfig()
		return nil
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("配置文件解析失败: %v", err)
	}
	if config.UI.SessionDays <= 0 {
		config.UI.SessionDays = 7
	}
	if config.Storage.MaxUploadMB <= 0 {
		config.Storage.MaxUploadMB = 1024
	}
	consoleLogger.Println("配置文件加载成功")
	return nil
}

func setDefaultConfig() {
	config.Server.Port = "8000"
	config.Server.ReadTimeout = 3600
	config.Server.WriteTimeout = 3600
	config.Storage.AudioDir = "./audio_files"
	config.Storage.MaxUploadMB = 1024
	config.UI.Password = "change-me"
	config.UI.SessionDays = 7
	config.Logging.Level = "INFO"
	config.Logging.LogDir = "./logs"
}
```

注意：`consoleLogger` 仍在 `main.go`（或一并移到 `logging.go`）；若编译报未定义，把 `consoleLogger`/`fileLogger` 声明留在 `main.go` 包级变量即可（同 package main）。

- [ ] **Step 2: 从 `main.go` 删除已迁出的类型与函数**，确认仍能编译。

Run: `go build -o bin/audio-server.exe .`
Expected: 成功，无错误。

- [ ] **Step 3: 写 `config.yaml.example`**

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

- [ ] **Step 4: Commit**

```bash
git add config.go config.yaml.example main.go
git commit -m "refactor: extract config and raise default upload limit"
```

---

### Task 2: 安全解压（zip / tar / tar.gz）

**Files:**
- Create: `extract.go`
- Create: `extract_test.go`

- [ ] **Step 1: 写失败测试 `extract_test.go`**

```go
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	createZip(t, zipPath, map[string]string{"../escape.txt": "pwned"})

	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	err := ExtractArchive(zipPath, dest)
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestExtractZip_OK(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "pkg.zip")
	createZip(t, zipPath, map[string]string{"a.txt": "hello"})
	dest := filepath.Join(dir, "pkg")
	_ = os.MkdirAll(dest, 0755)
	if err := ExtractArchive(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "a.txt"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("got %q err=%v", b, err)
	}
}

func TestArchiveBaseName(t *testing.T) {
	cases := map[string]string{
		"foo.zip":    "foo",
		"foo.tar":    "foo",
		"foo.tar.gz": "foo",
		"foo.tgz":    "foo",
		"bar.mp4":    "",
	}
	for in, want := range cases {
		if got := ArchiveBaseName(in); got != want {
			t.Fatalf("%s: got %q want %q", in, got, want)
		}
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

// 可选：再加一个 tar.gz 正向用例，结构类似，用 tar.Writer + gzip.Writer
var _ = tar.Writer{}
var _ = gzip.Writer{}
```

- [ ] **Step 2: Run 确认失败**

Run: `go test -run TestExtractZip_OK -v`
Expected: FAIL（`ExtractArchive` undefined）

- [ ] **Step 3: 实现 `extract.go`**

```go
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func ArchiveBaseName(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"):
		return filename[:len(filename)-7]
	case strings.HasSuffix(lower, ".tgz"):
		return filename[:len(filename)-4]
	case strings.HasSuffix(lower, ".tar"):
		return filename[:len(filename)-4]
	case strings.HasSuffix(lower, ".zip"):
		return filename[:len(filename)-4]
	default:
		return ""
	}
}

func ExtractArchive(archivePath, destDir string) error {
	lower := strings.ToLower(archivePath)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath, destDir)
	case strings.HasSuffix(lower, ".tar"):
		return extractTarFile(archivePath, destDir)
	default:
		return fmt.Errorf("unsupported archive type")
	}
}

func safeJoin(destDir, name string) (string, error) {
	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal path: %s", name)
	}
	target := filepath.Join(destDir, clean)
	rel, err := filepath.Rel(destDir, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("illegal path: %s", name)
	}
	return target, nil
}

func extractZip(path, destDir string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if err := writeZipFile(destDir, f); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(destDir string, f *zip.File) error {
	target, err := safeJoin(destDir, f.Name)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func extractTarGz(path, destDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return extractTar(tar.NewReader(gz), destDir)
}

func extractTarFile(path, destDir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return extractTar(tar.NewReader(f), destDir)
}

func extractTar(tr *tar.Reader, destDir string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeJoin(destDir, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add extract.go extract_test.go
git commit -m "feat: add safe zip/tar extraction helpers"
```

---

### Task 3: UI 密码 Cookie（不拦文件 API）

**Files:**
- Create: `ui_auth.go`
- Create: `ui_auth_test.go`

- [ ] **Step 1: 写测试**

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUISession_RoundTrip(t *testing.T) {
	config.UI.Password = "secret"
	config.UI.SessionDays = 7
	token := issueUISession(time.Now())
	if !validUISession(token, time.Now()) {
		t.Fatal("expected valid")
	}
	if validUISession(token, time.Now().Add(8*24*time.Hour)) {
		t.Fatal("expected expired")
	}
	if validUISession("tampered", time.Now()) {
		t.Fatal("expected invalid")
	}
}

func TestLoginHandler_SetsCookie(t *testing.T) {
	config.UI.Password = "secret"
	config.UI.SessionDays = 7
	req := httptest.NewRequest(http.MethodPost, "/api/ui/login", strings.NewReader(`{"password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	uiLoginHandler(rr, req)
	if rr.Code != 200 {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
	cookies := rr.Result().Cookies()
	if len(cookies) == 0 || cookies[0].Name != "ui_session" {
		t.Fatalf("missing cookie: %+v", cookies)
	}
}
```

- [ ] **Step 2: 实现 `ui_auth.go`**

Cookie 名：`ui_session`。值：`base64(expiryUnix|hmacSHA256(password, expiryUnix))`。  
`uiLoginHandler` / `uiLogoutHandler`；`requireUIAuth` 中间件仅包页面与 `/api/ui/*`（logout 除外可放行）。

```go
// 关键签名逻辑要点：
// msg := strconv.FormatInt(expiry.Unix(), 10)
// mac := hmac.New(sha256.New, []byte(config.UI.Password))
// mac.Write([]byte(msg))
// token := base64.RawURLEncoding.EncodeToString([]byte(msg + "|" + hex.EncodeToString(mac.Sum(nil))))
```

登录失败 401；成功 200 `{"success":true}`。Logout 清 Cookie。

- [ ] **Step 3: `go test -run TestUISession -count=1` 通过后 Commit**

```bash
git add ui_auth.go ui_auth_test.go
git commit -m "feat: add UI password session cookie"
```

---

### Task 4: 上传可选解压 + 新 list/deleteDir（兼容旧 API）

**Files:**
- Create: `handlers.go`（从 `main.go` 迁出全部 handler + 响应类型）
- Modify: upload 逻辑
- Create: `handlers_compat_test.go`

- [ ] **Step 1: 迁出 handlers 到 `handlers.go`**，`main.go` 只留 `main` + 中间件 + logger 初始化。

- [ ] **Step 2: 扩展 `UploadResponse`（仅附加可选字段）**

```go
type UploadResponse struct {
	Success      bool   `json:"success"`
	Filename     string `json:"filename,omitempty"`
	URL          string `json:"url,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ExtractedDir string `json:"extracted_dir,omitempty"`
	Error        string `json:"error,omitempty"`
}
```

- [ ] **Step 3: 修改 `uploadHandler`**

1. 保留 `MaxBytesReader` + `FormFile("file")` + `io.Copy`（已是流式从 multipart part 写出）。
2. `ParseMultipartForm` 的 memory 上限可保持，但大文件会落临时文件——可接受（≤1GB）。
3. 读取解压开关：

```go
extract := r.FormValue("extract") == "true" || r.URL.Query().Get("extract") == "true"
```

4. 仅当 `extract && ArchiveBaseName(filename) != ""` 时：

```go
dest := filepath.Join(config.Storage.AudioDir, ArchiveBaseName(filename))
_ = os.MkdirAll(dest, 0755)
if err := ExtractArchive(filePath, dest); err != nil {
    // 上传已成功：记日志，响应仍 success=true，可带 Error 说明解压失败，或返回 200 + extracted_dir 空
    // 推荐：上传成功、解压失败时 HTTP 200，success=true，另加 message；不要改掉旧客户端依赖的 success/filename/url/size
}
```

5. 不传 `extract` 时：响应 JSON **不得**依赖新字段；行为与旧版一致。

- [ ] **Step 4: 新增 `listFilesHandler` — `GET /api/files/list?path=&prefix=`**

- `path` 相对 `audio_dir`，禁止 `..`；空 = 根目录。
- 返回：

```json
{
  "success": true,
  "path": "",
  "entries": [
    {"name": "foo.zip", "type": "file", "size": 123, "url": "/audio/foo.zip"},
    {"name": "foo", "type": "dir", "size": 0, "url": ""}
  ]
}
```

子目录内文件 URL：`/audio/{path}/{name}`（`http.FileServer` 已支持嵌套）。

- [ ] **Step 5: 新增 `deleteDirHandler` — `DELETE /api/files/dir/{name}`**

- `{name}` 单层目录名，禁止 `..`/`/`。
- `os.RemoveAll` 删 `audio_dir/name`。
- **不要**改 `DELETE /api/files/{id}`。

- [ ] **Step 6: 兼容测试**

```go
func TestUploadWithoutExtract_NoExtractedDirSideEffect(t *testing.T) {
	// httptest 上传小文件不带 extract，目录下不应出现同名解压目录
}
func TestQueryVideos_UnchangedShape(t *testing.T) {
	// POST /api/files/query 空 filters，响应含 success + videos
}
```

- [ ] **Step 7: Commit**

```bash
git add handlers.go handlers_compat_test.go main.go
git commit -m "feat: optional extract on upload; add list and delete-dir APIs"
```

---

### Task 5: 内嵌 Web UI（登录、上传进度、列表、复制 URL）

**Files:**
- Create: `web/embed.go`, `web/login.html`, `web/index.html`, `web/app.js`, `web/style.css`
- Modify: `main.go` 路由

- [ ] **Step 1: `web/embed.go`**

```go
package web

import "embed"

//go:embed *.html *.js *.css
var Files embed.FS
```

注意：`main` 包引用 `audio-server/web`（模块名仍是 `audio-server`）。

- [ ] **Step 2: `login.html`** — 密码表单，POST JSON 到 `/api/ui/login`，成功跳转 `/`。

- [ ] **Step 3: `index.html` + `app.js` + `style.css`**

功能清单（对应 spec「够用」）：

1. 打开 `/` 时若无 Cookie，前端或服务端重定向 `/login.html`（推荐服务端：`uiPageHandler` 校验 Cookie）。
2. 拖拽/选择文件，`FormData` 追加 `file` + `extract=true`，用 `XMLHttpRequest.upload.onprogress` 显示进度条。
3. 调用 `GET /api/files/list?path=&prefix=` 渲染列表；点目录进入 `path`；前缀搜索输入框。
4. 每行：下载链接（`/audio/...`）、**复制 URL**（`navigator.clipboard.writeText(absoluteUrl)`）、删除（文件用 `DELETE /api/files/{id}`，目录用 `DELETE /api/files/dir/{name}`）。
5. 退出：`POST /api/ui/logout`。

UI 保持简单清晰即可，无需复杂设计系统。

- [ ] **Step 4: `main.go` 路由顺序（关键）**

```go
r.HandleFunc("/health", healthHandler).Methods("GET")
r.HandleFunc("/api/ui/login", uiLoginHandler).Methods("POST")
r.HandleFunc("/api/ui/logout", uiLogoutHandler).Methods("POST")
r.HandleFunc("/api/files/list", listFilesHandler).Methods("GET")
r.HandleFunc("/api/files/dir/{name}", deleteDirHandler).Methods("DELETE")
r.HandleFunc("/api/files", uploadHandler).Methods("POST")
r.HandleFunc("/api/files/query", queryVideosHandler).Methods("POST")
r.HandleFunc("/api/files/{id}/download", downloadVideoHandler).Methods("GET")
r.HandleFunc("/api/files/{id}", deleteFileHandler).Methods("DELETE")
r.PathPrefix("/audio/").Handler(...)

// UI pages — require cookie
r.HandleFunc("/", uiIndexHandler).Methods("GET")
r.HandleFunc("/login.html", uiLoginPageHandler).Methods("GET")
r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(web.Files))))
```

`uiIndexHandler`：无有效 Cookie → `302 /login.html`；有则返回 `index.html`。  
静态 js/css 可走 `/static/` 或同 embed 直接按名服务；**不要**给 `/api/files*` 挂 UI 鉴权中间件。

- [ ] **Step 5: 启用 `http.Server` 超时**

```go
srv := &http.Server{
	Addr:         addr,
	Handler:      handler,
	ReadTimeout:  time.Duration(config.Server.ReadTimeout) * time.Second,
	WriteTimeout: time.Duration(config.Server.WriteTimeout) * time.Second,
}
log.Fatal(srv.ListenAndServe())
```

- [ ] **Step 6: 本地手动烟测**

```bash
go build -o bin/audio-server.exe .
# 复制 config.yaml.example 为 config.yaml，改 password
./bin/audio-server.exe
# 浏览器打开 http://localhost:8000/ → 登录 → 上传小 zip（extract）→ 列表见目录 → 复制 URL
# curl 旧接口无 Cookie：
curl -s http://localhost:8000/health
curl -s -F "file=@test.bin" http://localhost:8000/api/files
```

- [ ] **Step 7: Commit**

```bash
git add web/ main.go
git commit -m "feat: embed password-gated file manager UI"
```

---

### Task 6: 文档与回归脚本

**Files:**
- Modify: `docs/API接口文档.md`（仅追加 § 新接口）
- Modify: `README.md`（简述 UI 登录与 `extract=true`）
- Modify: `scripts/test-api.sh`（保留旧用例；追加 list / extract 用例，且旧上传断言仍无强制 `extracted_dir`）

- [ ] **Step 1: API 文档追加**

- `GET /api/files/list`
- `DELETE /api/files/dir/{name}`
- `POST /api/files` 可选表单/query `extract=true` 与可选响应字段 `extracted_dir`
- `POST /api/ui/login` / `logout`
- 明确写：**未传 extract 时行为与 v2.0.0 一致**

- [ ] **Step 2: 跑 `go test ./...` 与（若环境有 bash）`scripts/test-api.sh`**

- [ ] **Step 3: Commit**

```bash
git add docs/API接口文档.md README.md scripts/test-api.sh
git commit -m "docs: document UI and optional extract APIs"
```

---

## Spec Coverage Checklist

| Spec 要求 | Task |
|-----------|------|
| ≤1GB 流式上传 / 提高上限 | 1, 4 |
| 可选自动解压 zip/tar | 2, 4 |
| 仅 UI 密码 | 3, 5 |
| 进度条 / 前缀搜索 / 复制 URL | 5 |
| 不破坏旧接口 | 4, 6 |
| Server 超时生效 | 5 |
| config.example | 1 |
| 保留原包 + 解压到同名目录 | 2, 4 |

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-07-17-nas-file-ui.md`.

**两种执行方式：**

1. **Subagent-Driven（推荐）** — 每个 Task 开新子代理，Task 间审查  
2. **Inline Execution** — 本会话按 executing-plans 连续做，带检查点  

你选哪一种？
