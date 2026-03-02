#!/bin/bash

# 音频文件服务器编译脚本（Linux版本）
# 用于交叉编译Linux AMD64版本，部署到阿里云ECS

set -e  # 遇到错误立即退出

echo "======================================"
echo "  音频文件服务器编译脚本 (Linux)"
echo "======================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 检查Go环境
echo -e "${YELLOW}检查Go环境...${NC}"
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: 未找到Go环境，请先安装Go 1.21+${NC}"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}')
echo -e "${GREEN}✓ Go版本: $GO_VERSION${NC}"
echo ""

# 配置Go代理（中国大陆使用国内镜像）
echo -e "${YELLOW}配置Go代理...${NC}"
export GOPROXY=https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct
export GO111MODULE=on
echo -e "${GREEN}✓ GOPROXY: $GOPROXY${NC}"
echo ""

# 下载依赖
echo -e "${YELLOW}下载依赖...${NC}"
go mod download
echo -e "${GREEN}✓ 依赖下载完成${NC}"
echo ""

# 清理旧的编译文件
echo -e "${YELLOW}清理旧的编译文件...${NC}"
rm -f bin/audio-server bin/audio-server.exe
mkdir -p bin/
echo -e "${GREEN}✓ 清理完成${NC}"
echo ""

# 编译Linux AMD64版本（用于ECS部署）
echo -e "${YELLOW}编译Linux AMD64版本...${NC}"
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

# 带优化的编译（减小可执行文件体积）
go build -ldflags="-s -w" -o bin/audio-server .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Linux AMD64版本编译成功${NC}"
else
    echo -e "${RED}✗ 编译失败${NC}"
    exit 1
fi
echo ""

# 显示编译结果
echo -e "${YELLOW}编译结果:${NC}"
ls -lh bin/audio-server
echo ""

# 显示文件信息
echo -e "${YELLOW}文件信息:${NC}"
file bin/audio-server
echo ""

# 计算文件大小
SIZE=$(du -h bin/audio-server | cut -f1)
echo -e "${GREEN}======================================"
echo "  编译成功！"
echo "  输出文件: bin/audio-server"
echo "  文件大小: $SIZE"
echo "  目标平台: Linux AMD64"
echo -e "======================================${NC}"
echo ""
echo -e "${YELLOW}部署步骤:${NC}"
echo "1. 上传到服务器: scp bin/audio-server root@your-ecs-ip:/root/"
echo "2. 登录服务器: ssh root@your-ecs-ip"
echo "3. 设置权限: chmod +x /root/audio-server"
echo "4. 配置systemd服务（参考部署指南）"
echo ""
