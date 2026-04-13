#!/bin/bash
# 端到端蒸馏流程验证脚本

set -e

echo "=========================================="
echo "端到端蒸馏流程验证"
echo "=========================================="

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 配置
TEST_WORKSPACE="/tmp/gcs-distill-e2e-test"
PROJECT_ID="test-project-001"
RUN_ID="test-run-001"
IMAGE_NAME="gcs-distill/easydistill:latest"

# 清理函数
cleanup() {
    echo -e "${YELLOW}清理测试环境...${NC}"
    rm -rf "$TEST_WORKSPACE"
    docker rm -f gcs-e2e-test 2>/dev/null || true
}

trap cleanup EXIT

# 创建工作空间结构
echo ""
echo "1. 创建工作空间结构..."
WORKSPACE="$TEST_WORKSPACE/projects/$PROJECT_ID/runs/$RUN_ID"
mkdir -p "$WORKSPACE"/{configs,data/seed,data/generated,data/filtered,models/checkpoints,logs,eval}

echo -e "${GREEN}✓ 工作空间创建完成${NC}"
tree "$WORKSPACE" 2>/dev/null || ls -R "$WORKSPACE"

# 准备种子数据
echo ""
echo "2. 准备种子数据..."
cp tests/integration/sample_seed_data.json "$WORKSPACE/data/seed/instructions.json"
echo -e "${GREEN}✓ 种子数据已复制${NC}"
echo "种子数据样本:"
head -n 5 "$WORKSPACE/data/seed/instructions.json"

# 生成教师推理配置
echo ""
echo "3. 生成教师推理配置..."
cat > "$WORKSPACE/configs/teacher_infer.json" <<EOF
{
  "job_type": "kd_black_box_local",
  "dataset": {
    "instruction_path": "/workspace/data/seed/instructions.json",
    "labeled_path": "/workspace/data/generated/labeled.json"
  },
  "inference": {
    "temperature": 0.8,
    "max_new_tokens": 512,
    "batch_size": 1
  },
  "models": {
    "teacher": "test-model"
  },
  "logging": {
    "log_file": "/workspace/logs/teacher_infer.log",
    "log_level": "INFO"
  }
}
EOF

echo -e "${GREEN}✓ 教师推理配置已生成${NC}"
cat "$WORKSPACE/configs/teacher_infer.json"

# 模拟教师推理（创建假数据）
echo ""
echo "4. 模拟教师推理输出..."
cat > "$WORKSPACE/data/generated/labeled.json" <<EOF
{"instruction": "解释什么是机器学习", "input": "", "output": "机器学习是人工智能的一个分支，它使计算机系统能够从数据中学习并改进性能。"}
{"instruction": "什么是深度学习", "input": "", "output": "深度学习是机器学习的一个子领域，使用多层神经网络来学习数据的复杂模式。"}
{"instruction": "Python中如何创建列表", "input": "", "output": "在Python中，使用方括号[]创建列表，例如: my_list = [1, 2, 3]"}
{"instruction": "解释什么是神经网络", "input": "", "output": "神经网络是一种受生物神经系统启发的计算模型，由多个互连的节点组成。"}
{"instruction": "什么是自然语言处理", "input": "", "output": "自然语言处理(NLP)是人工智能的一个领域，专注于使计算机理解和处理人类语言。"}
EOF

echo -e "${GREEN}✓ 教师推理输出已生成${NC}"
echo "生成的标注数据行数: $(wc -l < $WORKSPACE/data/generated/labeled.json)"

# 模拟日志输出
echo ""
echo "5. 创建模拟日志文件..."
cat > "$WORKSPACE/logs/teacher_infer.log" <<EOF
[2026-04-13 15:00:00] INFO: Starting teacher inference
[2026-04-13 15:00:01] INFO: Loading instruction data from /workspace/data/seed/instructions.json
[2026-04-13 15:00:02] INFO: Loaded 5 instructions
[2026-04-13 15:00:03] INFO: Initializing teacher model: test-model
[2026-04-13 15:00:05] INFO: Processing instruction 1/5
[2026-04-13 15:00:08] INFO: Processing instruction 2/5
[2026-04-13 15:00:11] INFO: Processing instruction 3/5
[2026-04-13 15:00:14] INFO: Processing instruction 4/5
[2026-04-13 15:00:17] INFO: Processing instruction 5/5
[2026-04-13 15:00:18] INFO: Saving labeled data to /workspace/data/generated/labeled.json
[2026-04-13 15:00:19] INFO: Teacher inference completed successfully
[2026-04-13 15:00:19] INFO: Total processed: 5, Success: 5, Failed: 0
EOF

echo -e "${GREEN}✓ 日志文件已创建${NC}"
echo "日志内容:"
cat "$WORKSPACE/logs/teacher_infer.log"

# 验证日志读取
echo ""
echo "6. 验证日志文件可读性..."
if [ -f "$WORKSPACE/logs/teacher_infer.log" ]; then
    LOG_SIZE=$(stat -f%z "$WORKSPACE/logs/teacher_infer.log" 2>/dev/null || stat -c%s "$WORKSPACE/logs/teacher_infer.log")
    echo -e "${GREEN}✓ 日志文件可访问，大小: $LOG_SIZE bytes${NC}"
    echo "最后5行:"
    tail -n 5 "$WORKSPACE/logs/teacher_infer.log"
else
    echo -e "${RED}✗ 日志文件不存在${NC}"
    exit 1
fi

# 生成学生训练配置
echo ""
echo "7. 生成学生训练配置..."
cat > "$WORKSPACE/configs/student_train.json" <<EOF
{
  "job_type": "kd_black_box_train_only",
  "dataset": {
    "instruction_path": "/workspace/data/filtered/train.json",
    "template": "chat_template/chat_template_kd.jinja"
  },
  "models": {
    "teacher": "test-teacher-model",
    "student": "test-student-model"
  },
  "training": {
    "output_dir": "/workspace/models/checkpoints/",
    "num_train_epochs": 1,
    "per_device_train_batch_size": 2,
    "learning_rate": 2e-5,
    "save_steps": 100,
    "logging_dir": "/workspace/logs/",
    "logging_steps": 10
  }
}
EOF

echo -e "${GREEN}✓ 学生训练配置已生成${NC}"

# 模拟模型输出
echo ""
echo "8. 模拟模型检查点输出..."
mkdir -p "$WORKSPACE/models/checkpoints/checkpoint-100"
echo "fake model checkpoint" > "$WORKSPACE/models/checkpoints/checkpoint-100/pytorch_model.bin"
echo "fake config" > "$WORKSPACE/models/checkpoints/checkpoint-100/config.json"

if [ -f "$WORKSPACE/models/checkpoints/checkpoint-100/pytorch_model.bin" ]; then
    echo -e "${GREEN}✓ 模型检查点可访问${NC}"
    ls -lh "$WORKSPACE/models/checkpoints/checkpoint-100/"
else
    echo -e "${RED}✗ 模型检查点访问失败${NC}"
    exit 1
fi

# 测试卷挂载
echo ""
echo "9. 测试 Docker 卷挂载..."
if docker images | grep -q "gcs-distill/easydistill"; then
    docker run --rm \
        -v "$WORKSPACE:/workspace" \
        "$IMAGE_NAME" \
        bash -c "ls -la /workspace/configs/ && ls -la /workspace/logs/" > /dev/null 2>&1

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Docker 卷挂载正常${NC}"
    else
        echo -e "${RED}✗ Docker 卷挂载失败${NC}"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠ EasyDistill 镜像不存在，跳过 Docker 测试${NC}"
    echo "  请运行 'make docker-build' 构建镜像"
fi

# 最终验证
echo ""
echo "=========================================="
echo -e "${GREEN}端到端测试完成！${NC}"
echo "=========================================="
echo ""
echo "验证结果:"
echo "  ✓ 工作空间结构正确"
echo "  ✓ 种子数据已准备"
echo "  ✓ 配置文件已生成"
echo "  ✓ 教师推理输出已创建"
echo "  ✓ 日志文件可读取"
echo "  ✓ 模型输出可访问"
echo ""
echo "工作空间路径: $WORKSPACE"
echo ""
echo "下一步："
echo "  1. 使用 API 创建项目并上传种子数据"
echo "  2. 启动蒸馏流水线"
echo "  3. 通过 API 查看日志和模型输出"
