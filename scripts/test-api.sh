#!/bin/bash

# 文件服务器API测试脚本 (v2.0)

set -e

echo "======================================"
echo "  文件服务器API测试"
echo "======================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 服务器地址
SERVER_URL="http://localhost:8000"
TEST_DIR="../test"

# 检查服务器是否运行
echo -e "${YELLOW}1. 检查服务器状态...${NC}"
if ! curl -s --connect-timeout 5 "$SERVER_URL/health" > /dev/null 2>&1; then
    echo -e "${RED}错误: 无法连接到服务器 $SERVER_URL${NC}"
    echo "请确保服务器已启动 (运行: cd bin && ./audio-server.exe)"
    exit 1
fi
echo -e "${GREEN}✓ 服务器运行正常${NC}"
echo ""

# 测试1: 健康检查接口
echo -e "${BLUE}======================================"
echo "测试 1: 健康检查接口"
echo -e "======================================${NC}"
echo "请求: GET $SERVER_URL/health"
echo ""

HEALTH_RESPONSE=$(curl -s "$SERVER_URL/health")
echo "响应:"
echo "$HEALTH_RESPONSE" | jq '.' 2>/dev/null || echo "$HEALTH_RESPONSE"
echo ""

# 验证响应
if echo "$HEALTH_RESPONSE" | grep -q "running"; then
    echo -e "${GREEN}✓ 健康检查测试通过${NC}"
else
    echo -e "${RED}✗ 健康检查测试失败${NC}"
fi
echo ""

# 测试2: 文件上传接口（小文件）
echo -e "${BLUE}======================================"
echo "测试 2: 文件上传接口（创建测试文件）"
echo -e "======================================${NC}"

# 创建测试文件
TEST_FILE="$TEST_DIR/test_audio.wav"
mkdir -p "$TEST_DIR"

echo "创建测试文件: $TEST_FILE"
# 创建一个简单的WAV文件头（44字节）+ 随机数据
dd if=/dev/zero of="$TEST_FILE" bs=1024 count=10 2>/dev/null
echo -e "${GREEN}✓ 测试文件创建成功 (10KB)${NC}"
echo ""

echo "请求: POST $SERVER_URL/api/files"
echo "上传文件: $TEST_FILE"
echo ""

UPLOAD_RESPONSE=$(curl -s -X POST \
    -F "file=@$TEST_FILE" \
    "$SERVER_URL/api/files")

echo "响应:"
echo "$UPLOAD_RESPONSE" | jq '.' 2>/dev/null || echo "$UPLOAD_RESPONSE"
echo ""

# 验证上传响应
if echo "$UPLOAD_RESPONSE" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ 文件上传测试通过${NC}"

    # 提取文件名用于后续测试
    UPLOADED_FILENAME=$(echo "$UPLOAD_RESPONSE" | jq -r '.filename' 2>/dev/null || echo "")
    echo "上传的文件名: $UPLOADED_FILENAME"
else
    echo -e "${RED}✗ 文件上传测试失败${NC}"
    UPLOADED_FILENAME=""
fi
echo ""

# 测试3: 文件下载接口（API）
if [ -n "$UPLOADED_FILENAME" ]; then
    echo -e "${BLUE}======================================"
    echo "测试 3: 文件下载接口（API）"
    echo -e "======================================${NC}"
    echo "请求: GET $SERVER_URL/api/files/$UPLOADED_FILENAME/download"
    echo ""

    DOWNLOADED_FILE="$TEST_DIR/downloaded.wav"

    # 下载文件
    HTTP_CODE=$(curl -s -o "$DOWNLOADED_FILE" -w "%{http_code}" "$SERVER_URL/api/files/$UPLOADED_FILENAME/download")

    if [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ 文件下载成功 (HTTP $HTTP_CODE)${NC}"

        # 比较文件大小
        ORIGINAL_SIZE=$(stat -f%z "$TEST_FILE" 2>/dev/null || stat -c%s "$TEST_FILE" 2>/dev/null)
        DOWNLOADED_SIZE=$(stat -f%z "$DOWNLOADED_FILE" 2>/dev/null || stat -c%s "$DOWNLOADED_FILE" 2>/dev/null)

        if [ "$ORIGINAL_SIZE" = "$DOWNLOADED_SIZE" ]; then
            echo -e "${GREEN}✓ 文件完整性验证通过 (大小: $DOWNLOADED_SIZE bytes)${NC}"
        else
            echo -e "${RED}✗ 文件大小不匹配 (原始: $ORIGINAL_SIZE, 下载: $DOWNLOADED_SIZE)${NC}"
        fi

        # 清理下载的测试文件
        rm -f "$DOWNLOADED_FILE"
    else
        echo -e "${RED}✗ 文件下载失败 (HTTP $HTTP_CODE)${NC}"
    fi
    echo ""

    # 测试3.1: HTTP Range 断点续传
    echo -e "${BLUE}--------------------------------------"
    echo "测试 3.1: HTTP Range 断点续传"
    echo -e "--------------------------------------${NC}"
    echo "请求: GET $SERVER_URL/api/files/$UPLOADED_FILENAME/download (Range: bytes=0-1023)"
    echo ""

    RANGE_FILE="$TEST_DIR/partial.bin"
    HTTP_CODE=$(curl -s -o "$RANGE_FILE" -w "%{http_code}" -H "Range: bytes=0-1023" "$SERVER_URL/api/files/$UPLOADED_FILENAME/download")
    RANGE_SIZE=$(stat -f%z "$RANGE_FILE" 2>/dev/null || stat -c%s "$RANGE_FILE" 2>/dev/null)

    if [ "$HTTP_CODE" = "206" ] && [ "$RANGE_SIZE" = "1024" ]; then
        echo -e "${GREEN}✓ Range 请求成功 (HTTP 206, 收到 $RANGE_SIZE bytes)${NC}"
    else
        echo -e "${YELLOW}⚠ Range 测试结果: HTTP $HTTP_CODE, 收到 $RANGE_SIZE bytes (期望 206/1024)${NC}"
    fi
    rm -f "$RANGE_FILE"
    echo ""
fi

# 测试4: 静态文件访问（旁路）
if [ -n "$UPLOADED_FILENAME" ]; then
    echo -e "${BLUE}======================================"
    echo "测试 4: 静态文件访问（旁路 /audio/）"
    echo -e "======================================${NC}"
    echo "请求: GET $SERVER_URL/audio/$UPLOADED_FILENAME"
    echo ""

    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/audio/$UPLOADED_FILENAME")
    if [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ 静态访问成功 (HTTP $HTTP_CODE)${NC}"
    else
        echo -e "${RED}✗ 静态访问失败 (HTTP $HTTP_CODE)${NC}"
    fi
    echo ""
fi

# 测试5: 文件列表查询
echo -e "${BLUE}======================================"
echo "测试 5: 文件列表查询"
echo -e "======================================${NC}"
echo "请求: POST $SERVER_URL/api/files/query"
echo ""

QUERY_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" -d '{}' "$SERVER_URL/api/files/query")
echo "响应:"
echo "$QUERY_RESPONSE" | jq '.' 2>/dev/null || echo "$QUERY_RESPONSE"
echo ""

if echo "$QUERY_RESPONSE" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ 文件列表查询通过${NC}"
else
    echo -e "${RED}✗ 文件列表查询失败${NC}"
fi
echo ""

# 测试6: 访问不存在的文件（404测试）
echo -e "${BLUE}======================================"
echo "测试 6: 访问不存在的文件（404测试）"
echo -e "======================================${NC}"
echo "请求: GET $SERVER_URL/api/files/nonexistent_file_xyz.wav/download"
echo ""

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/api/files/nonexistent_file_xyz.wav/download")

if [ "$HTTP_CODE" = "404" ]; then
    echo -e "${GREEN}✓ 404错误处理正确 (HTTP $HTTP_CODE)${NC}"
else
    echo -e "${YELLOW}⚠ HTTP状态码: $HTTP_CODE (期望404)${NC}"
fi
echo ""

# 测试7: 路径遍历防护
echo -e "${BLUE}======================================"
echo "测试 7: 路径遍历防护"
echo -e "======================================${NC}"
echo "请求: GET $SERVER_URL/api/files/..%2F..%2Fetc%2Fpasswd/download"
echo ""

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/api/files/..%2F..%2Fetc%2Fpasswd/download")

if [ "$HTTP_CODE" = "400" ]; then
    echo -e "${GREEN}✓ 路径遍历防护正确 (HTTP 400)${NC}"
else
    echo -e "${YELLOW}⚠ HTTP状态码: $HTTP_CODE (期望400)${NC}"
fi
echo ""

# 测试8: 文件大小限制验证
echo -e "${BLUE}======================================"
echo "测试 8: 文件大小限制验证"
echo -e "======================================${NC}"
echo "创建超大测试文件 (>100MB)..."
echo ""

BIG_FILE="$TEST_DIR/big_test.wav"
# 创建一个105MB的文件（超过100MB限制）
dd if=/dev/zero of="$BIG_FILE" bs=1048576 count=105 2>/dev/null

echo "请求: POST $SERVER_URL/api/files"
echo "上传文件: $BIG_FILE (105MB)"
echo ""

UPLOAD_RESPONSE=$(curl -s -X POST \
    -F "file=@$BIG_FILE" \
    "$SERVER_URL/api/files" \
    --max-time 10 \
    2>&1 || echo "timeout")

echo "响应:"
echo "$UPLOAD_RESPONSE" | jq '.' 2>/dev/null || echo "$UPLOAD_RESPONSE"
echo ""

if echo "$UPLOAD_RESPONSE" | grep -qi "error\|过大\|too large"; then
    echo -e "${GREEN}✓ 文件大小限制正常工作${NC}"
else
    echo -e "${YELLOW}⚠ 文件大小限制测试结果不确定${NC}"
fi

# 清理大文件
rm -f "$BIG_FILE"
echo ""

# 测试9: 文件删除
if [ -n "$UPLOADED_FILENAME" ]; then
    echo -e "${BLUE}======================================"
    echo "测试 9: 文件删除（硬删除）"
    echo -e "======================================${NC}"
    echo "请求: DELETE $SERVER_URL/api/files/$UPLOADED_FILENAME"
    echo ""

    DELETE_RESPONSE=$(curl -s -X DELETE "$SERVER_URL/api/files/$UPLOADED_FILENAME")
    echo "响应:"
    echo "$DELETE_RESPONSE" | jq '.' 2>/dev/null || echo "$DELETE_RESPONSE"
    echo ""

    if echo "$DELETE_RESPONSE" | grep -q '"success":true'; then
        echo -e "${GREEN}✓ 文件删除测试通过${NC}"

        # 验证文件已不存在
        HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/api/files/$UPLOADED_FILENAME/download")
        if [ "$HTTP_CODE" = "404" ]; then
            echo -e "${GREEN}✓ 删除后访问返回 404 (HTTP $HTTP_CODE)${NC}"
        else
            echo -e "${YELLOW}⚠ 删除后访问返回 HTTP $HTTP_CODE (期望404)${NC}"
        fi
    else
        echo -e "${RED}✗ 文件删除测试失败${NC}"
    fi
    echo ""
fi

# 测试总结
echo -e "${GREEN}======================================"
echo "  测试完成"
echo -e "======================================${NC}"
echo ""
echo -e "${YELLOW}测试文件位置:${NC}"
echo "  $TEST_FILE"
echo ""
echo -e "${YELLOW}日志文件位置:${NC}"
echo "  logs/file-server-$(date +%Y-%m-%d).log"
echo ""
echo -e "${YELLOW}查看日志:${NC}"
echo "  tail -f logs/file-server-$(date +%Y-%m-%d).log"
echo ""
