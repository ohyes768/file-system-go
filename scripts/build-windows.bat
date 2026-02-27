@echo off
REM ======================================
REM Audio File Server Build Script (Windows)
REM Cross-compile for Linux AMD64 (ECS deployment)
REM ======================================

REM Change to project root directory
cd /d "%~dp0.."

echo.
echo ======================================
echo   Audio File Server Build Script
echo   Target: Linux AMD64
echo ======================================
echo.

REM Check Go environment
echo [1/5] Checking Go environment...
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go not found, please install Go 1.21+
    pause
    exit /b 1
)

for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
echo [OK] Go version: %GO_VERSION%
echo.

REM Set build environment
echo [2/5] Setting build environment...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
set GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
set GO111MODULE=on
echo [OK] Target platform: Linux AMD64
echo.

REM Download dependencies
echo [3/5] Downloading dependencies...
go mod download
if %errorlevel% neq 0 (
    echo [ERROR] Failed to download dependencies
    pause
    exit /b 1
)
echo [OK] Dependencies downloaded
echo.

REM Clean old files
echo [4/5] Cleaning old files...
if exist bin\audio-server del /f /q bin\audio-server 2>nul
if not exist bin mkdir bin
echo [OK] Cleaned
echo.

REM Build
echo [5/5] Building Linux AMD64 version...
go build -ldflags="-s -w" -o bin\audio-server main.go
if %errorlevel% neq 0 (
    echo [ERROR] Build failed
    pause
    exit /b 1
)
echo [OK] Build completed
echo.

REM Show result
echo ======================================
echo   Build SUCCESS!
echo ======================================
dir bin\audio-server
echo.

echo Deployment steps:
echo   1. Upload: scp bin\audio-server root@your-ecs-ip:/root/
echo   2. Login: ssh root@your-ecs-ip
echo   3. Permission: chmod +x /root/audio-server
echo   4. Restart: systemctl restart audio-file-server
echo.
pause
