#!/bin/bash

# 音频文件服务器本地测试编译脚本（Windows版本）
# 用于在本地Windows环境编译和测试

set -e  # 遇到错误立即退出

echo "======================================"
echo "  音频文件服务器本地编译 (Windows)"
echo "======================================"
echo ""

# 颜色定义（Windows Git Bash支持）
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
rm -rf bin/
mkdir -p bin/
echo -e "${GREEN}✓ 清理完成${NC}"
echo ""

# 编译Windows版本
echo -e "${YELLOW}编译Windows AMD64版本...${NC}"
export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=0

# 带优化的编译
go build -ldflags="-s -w" -o bin/audio-server.exe main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Windows版本编译成功${NC}"
else
    echo -e "${RED}✗ 编译失败${NC}"
    exit 1
fi
echo ""

# 显示编译结果
echo -e "${YELLOW}编译结果:${NC}"
ls -lh bin/audio-server.exe
echo ""

# 创建启动脚本
echo -e "${YELLOW}创建启动脚本...${NC}"
cat > bin/start-server.sh << 'EOF'
#!/bin/bash
# 启动音频文件服务器

echo "启动音频文件服务器..."
./audio-server.exe
EOF

chmod +x bin/start-server.sh
echo -e "${GREEN}✓ 启动脚本创建成功${NC}"
echo ""

echo -e "${GREEN}======================================"
echo "  编译成功！"
echo "  输出文件: bin/audio-server.exe"
echo "  启动脚本: bin/start-server.sh"
echo "  目标平台: Windows AMD64"
echo -e "======================================${NC}"
echo ""
echo -e "${YELLOW}本地测试步骤:${NC}"
echo "1. 进入bin目录: cd bin"
echo "2. 启动服务器: ./audio-server.exe"
echo "3. 或使用启动脚本: ./start-server.sh"
echo "4. 访问健康检查: curl http://localhost:8000/"
echo "5. 测试上传: curl -X POST -F \"file=@test.wav\" http://localhost:8000/upload"
echo ""
echo -e "${YELLOW}在新的终端窗口运行测试脚本:${NC}"
echo "bash scripts/test-api.sh"
echo ""
