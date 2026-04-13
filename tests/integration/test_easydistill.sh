#!/bin/bash
# EasyDistill 容器集成测试脚本

set -e

echo "=========================================="
echo "EasyDistill 容器集成测试"
echo "=========================================="

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 配置
IMAGE_NAME="gcs-distill/easydistill:latest"
TEST_WORKSPACE="/tmp/gcs-distill-test-workspace"

# 清理函数
cleanup() {
    echo -e "${YELLOW}清理测试环境...${NC}"
    rm -rf "$TEST_WORKSPACE"
    docker rm -f gcs-test-container 2>/dev/null || true
}

# 注册清理函数
trap cleanup EXIT

# 1. 测试镜像是否存在
echo ""
echo "1. 检查 EasyDistill 镜像..."
if docker images | grep -q "gcs-distill/easydistill"; then
    echo -e "${GREEN}✓ 镜像存在${NC}"
else
    echo -e "${RED}✗ 镜像不存在，请先运行: make docker-build${NC}"
    exit 1
fi

# 2. 测试镜像基本功能
echo ""
echo "2. 测试镜像基本功能..."
if docker run --rm "$IMAGE_NAME" --help > /dev/null 2>&1; then
    echo -e "${GREEN}✓ 镜像可以正常运行${NC}"
else
    echo -e "${RED}✗ 镜像运行失败${NC}"
    exit 1
fi

# 3. 测试工作空间挂载
echo ""
echo "3. 测试工作空间挂载..."
mkdir -p "$TEST_WORKSPACE"/{configs,data,models,logs}

# 创建测试配置文件
cat > "$TEST_WORKSPACE/configs/test.json" <<EOF
{
  "job_type": "test",
  "test_mode": true
}
EOF

# 测试卷挂载是否正确
docker run --rm \
    -v "$TEST_WORKSPACE:/workspace" \
    "$IMAGE_NAME" \
    bash -c "ls -la /workspace/configs/test.json" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 卷挂载正常${NC}"
else
    echo -e "${RED}✗ 卷挂载失败${NC}"
    exit 1
fi

# 4. 测试日志输出
echo ""
echo "4. 测试日志输出到文件..."

# 创建一个简单的测试脚本
cat > "$TEST_WORKSPACE/test_log.py" <<EOF
import sys
print("Test log output to stdout")
print("Test log output to stderr", file=sys.stderr)
with open("/workspace/logs/test.log", "w") as f:
    f.write("Test log output to file\\n")
EOF

docker run --rm \
    -v "$TEST_WORKSPACE:/workspace" \
    "$IMAGE_NAME" \
    python /workspace/test_log.py

if [ -f "$TEST_WORKSPACE/logs/test.log" ]; then
    echo -e "${GREEN}✓ 日志文件写入成功${NC}"
    echo "日志内容: $(cat $TEST_WORKSPACE/logs/test.log)"
else
    echo -e "${RED}✗ 日志文件写入失败${NC}"
    exit 1
fi

# 5. 测试模型输出目录
echo ""
echo "5. 测试模型输出目录..."

docker run --rm \
    -v "$TEST_WORKSPACE:/workspace" \
    "$IMAGE_NAME" \
    bash -c "echo 'fake model' > /workspace/models/test_model.bin"

if [ -f "$TEST_WORKSPACE/models/test_model.bin" ]; then
    echo -e "${GREEN}✓ 模型输出目录可访问${NC}"
else
    echo -e "${RED}✗ 模型输出目录访问失败${NC}"
    exit 1
fi

# 6. 测试 GPU 支持（如果可用）
echo ""
echo "6. 测试 GPU 支持..."
if command -v nvidia-smi &> /dev/null; then
    if docker run --rm --gpus all "$IMAGE_NAME" bash -c "nvidia-smi" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ GPU 支持正常${NC}"
    else
        echo -e "${YELLOW}⚠ GPU 可用但容器内访问失败${NC}"
    fi
else
    echo -e "${YELLOW}⚠ 未检测到 NVIDIA GPU，跳过 GPU 测试${NC}"
fi

# 7. 测试 EasyDistill 命令
echo ""
echo "7. 测试 EasyDistill 命令..."
docker run --rm "$IMAGE_NAME" --version > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ EasyDistill 命令可用${NC}"
else
    echo -e "${YELLOW}⚠ EasyDistill 版本命令不支持（可能是正常的）${NC}"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}所有测试通过！${NC}"
echo "=========================================="
echo ""
echo "测试工作空间内容:"
ls -la "$TEST_WORKSPACE"
echo ""
echo "子目录内容:"
for dir in configs data models logs; do
    echo "  $dir/:"
    ls -la "$TEST_WORKSPACE/$dir" 2>/dev/null | sed 's/^/    /' || echo "    (empty)"
done
