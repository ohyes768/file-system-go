#!/bin/bash

# 文件服务器部署脚本
# 用法: bash scripts/deploy.sh

set -e

DEPLOY_USER="ohyes768"
DEPLOY_HOST=""
DEPLOY_PATH="/home/ohyes768/file-system-go"

echo "======================================"
echo "  文件服务器部署脚本"
echo "======================================"
echo ""

# 检查参数
if [ -z "$1" ]; then
    read -p "请输入服务器 IP 地址: " DEPLOY_HOST
else
    DEPLOY_HOST="$1"
fi

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# 1. 编译 Linux 版本
echo -e "${YELLOW}[1/4] 编译 Linux AMD64 版本...${NC}"
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0
go build -ldflags="-s -w" -o bin/audio-server .
echo -e "${GREEN}✓ 编译完成${NC}"

# 2. 上传到服务器
echo -e "${YELLOW}[2/4] 上传到服务器...${NC}"
scp bin/audio-server config.yaml ${DEPLOY_USER}@${DEPLOY_HOST}:${DEPLOY_PATH}/
echo -e "${GREEN}✓ 上传完成${NC}"

# 3. 创建目录和设置权限
echo -e "${YELLOW}[3/4] 配置服务器...${NC}"
ssh ${DEPLOY_USER}@${DEPLOY_HOST} "cd ${DEPLOY_PATH} && \
    mkdir -p audio_files logs && \
    chmod +x audio-server"
echo -e "${GREEN}✓ 服务器配置完成${NC}"

# 4. 重启服务
echo -e "${YELLOW}[4/4] 重启服务...${NC}"
ssh ${DEPLOY_USER}@${DEPLOY_HOST} "sudo systemctl restart audio-file-server"
echo -e "${GREEN}✓ 服务已重启${NC}"

# 验证
echo ""
echo -e "${YELLOW}验证部署...${NC}"
sleep 2
ssh ${DEPLOY_USER}@${DEPLOY_HOST} "curl -s http://localhost:8000/health"

echo ""
echo -e "${GREEN}======================================"
echo "  部署完成！"
echo "  服务地址: http://${DEPLOY_HOST}:8000"
echo -e "======================================${NC}"