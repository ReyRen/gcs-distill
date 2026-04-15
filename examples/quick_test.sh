#!/bin/bash
# 快速测试脚本 - 使用种子数据创建蒸馏项目
# Quick test script - Create distillation project with seed data

set -e

# 配置
API_URL="${API_URL:-http://172.18.36.230:18080}"
SEED_FILE="${1:-examples/seed_data_customer_service.jsonl}"

echo "🚀 GCS-Distill 快速测试脚本"
echo "===================================="
echo "API URL: $API_URL"
echo "种子数据: $SEED_FILE"
echo ""

# 检查种子文件是否存在
if [ ! -f "$SEED_FILE" ]; then
    echo "❌ 错误: 种子文件不存在: $SEED_FILE"
    echo ""
    echo "可用的种子数据文件:"
    ls -1 examples/*.jsonl 2>/dev/null || echo "  (未找到)"
    exit 1
fi

# 步骤1: 创建项目
echo "📝 步骤 1/4: 创建蒸馏项目..."
PROJECT_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/projects" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试蒸馏项目",
    "description": "使用示例种子数据的测试项目",
    "teacher_model": {
      "model_name": "Qwen/Qwen2.5-7B-Instruct",
      "provider_type": "local"
    },
    "student_model": {
      "model_name": "Qwen/Qwen2.5-0.5B-Instruct"
    }
  }')

PROJECT_ID=$(echo "$PROJECT_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)

if [ -z "$PROJECT_ID" ]; then
    echo "❌ 项目创建失败"
    echo "$PROJECT_RESPONSE"
    exit 1
fi

echo "✅ 项目创建成功! ID: $PROJECT_ID"

# 步骤2: 上传数据集
echo ""
echo "📤 步骤 2/4: 上传种子数据集..."
DATASET_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/projects/$PROJECT_ID/datasets" \
  -F "file=@$SEED_FILE")

DATASET_ID=$(echo "$DATASET_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)

if [ -z "$DATASET_ID" ]; then
    echo "❌ 数据集上传失败"
    echo "$DATASET_RESPONSE"
    exit 1
fi

echo "✅ 数据集上传成功! ID: $DATASET_ID"

# 步骤3: 启动流水线
echo ""
echo "🔧 步骤 3/4: 启动蒸馏流水线..."
PIPELINE_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/pipelines" \
  -H "Content-Type: application/json" \
  -d "{
    \"project_id\": $PROJECT_ID,
    \"dataset_id\": $DATASET_ID,
    \"training_config\": {
      \"num_train_epochs\": 3,
      \"learning_rate\": 2e-5,
      \"per_device_train_batch_size\": 4
    }
  }")

PIPELINE_ID=$(echo "$PIPELINE_RESPONSE" | grep -o '"id":[0-9]*' | head -1 | cut -d':' -f2)

if [ -z "$PIPELINE_ID" ]; then
    echo "❌ 流水线启动失败"
    echo "$PIPELINE_RESPONSE"
    exit 1
fi

echo "✅ 流水线启动成功! ID: $PIPELINE_ID"

# 步骤4: 查看状态
echo ""
echo "📊 步骤 4/4: 查看流水线状态..."
sleep 2
STATUS_RESPONSE=$(curl -s "$API_URL/api/v1/pipelines/$PIPELINE_ID")
echo "$STATUS_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$STATUS_RESPONSE"

echo ""
echo "===================================="
echo "✅ 测试完成!"
echo ""
echo "项目 ID:    $PROJECT_ID"
echo "数据集 ID:  $DATASET_ID"
echo "流水线 ID:  $PIPELINE_ID"
echo ""
echo "查看流水线详情:"
echo "  curl $API_URL/api/v1/pipelines/$PIPELINE_ID"
echo ""
echo "查看阶段详情:"
echo "  curl $API_URL/api/v1/pipelines/$PIPELINE_ID/stages"
echo ""
echo "查看日志:"
echo "  curl $API_URL/api/v1/pipelines/$PIPELINE_ID/logs"
