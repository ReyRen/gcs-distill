#!/bin/bash

# GCS-Distill API 综合测试脚本
# 用于测试所有核心接口的功能

set -e

BASE_URL="http://localhost:18080"
API_BASE="${BASE_URL}/api/v1"

echo "=========================================="
echo "GCS-Distill API 综合测试"
echo "=========================================="
echo ""

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 测试结果函数
test_pass() {
    echo -e "${GREEN}✓ PASS${NC} - $1"
    ((PASSED_TESTS++))
    ((TOTAL_TESTS++))
}

test_fail() {
    echo -e "${RED}✗ FAIL${NC} - $1"
    echo "  Error: $2"
    ((FAILED_TESTS++))
    ((TOTAL_TESTS++))
}

test_info() {
    echo -e "${YELLOW}ℹ INFO${NC} - $1"
}

# 等待服务启动
echo "Step 0: 检查服务健康状态..."
echo "----------------------------------------"
MAX_RETRIES=30
RETRY_COUNT=0
while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s "${BASE_URL}/health" > /dev/null 2>&1; then
        HEALTH_RESPONSE=$(curl -s "${BASE_URL}/health")
        echo "Health check response: $HEALTH_RESPONSE"
        test_pass "服务健康检查"
        break
    else
        ((RETRY_COUNT++))
        if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
            test_fail "服务健康检查" "服务在${MAX_RETRIES}次重试后仍未响应"
            echo "请先启动服务: docker-compose up -d"
            exit 1
        fi
        echo "等待服务启动... ($RETRY_COUNT/$MAX_RETRIES)"
        sleep 2
    fi
done
echo ""

# ============================================
# 第一部分：项目管理 API 测试
# ============================================
echo "第一部分：项目管理 API 测试"
echo "=========================================="

# 测试 1.1: 创建项目
echo ""
echo "测试 1.1: 创建项目"
echo "----------------------------------------"
PROJECT_DATA='{
  "name": "测试蒸馏项目",
  "description": "将 Qwen2.5-7B 蒸馏到 Qwen2.5-0.5B 的测试项目",
  "business_scenario": "智能客服问答",
  "teacher_model_config": {
    "provider_type": "local",
    "model_name": "Qwen/Qwen2.5-7B-Instruct",
    "temperature": 0.7,
    "max_tokens": 2048,
    "concurrency": 10
  },
  "student_model_config": {
    "provider_type": "local",
    "model_name": "Qwen/Qwen2.5-0.5B-Instruct"
  },
  "evaluation_config": {
    "metrics": ["bleu", "rouge", "accuracy"],
    "test_set_ratio": 0.2
  }
}'

CREATE_PROJECT_RESPONSE=$(curl -s -X POST "${API_BASE}/projects" \
  -H "Content-Type: application/json" \
  -d "$PROJECT_DATA")

echo "Response: $CREATE_PROJECT_RESPONSE"
PROJECT_ID=$(echo "$CREATE_PROJECT_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ ! -z "$PROJECT_ID" ]; then
    test_pass "创建项目成功，ID: $PROJECT_ID"
else
    test_fail "创建项目" "未能获取项目ID"
    echo "Response: $CREATE_PROJECT_RESPONSE"
fi

# 测试 1.2: 获取项目列表
echo ""
echo "测试 1.2: 获取项目列表"
echo "----------------------------------------"
LIST_PROJECTS_RESPONSE=$(curl -s "${API_BASE}/projects?page=1&page_size=10")
echo "Response: $LIST_PROJECTS_RESPONSE"

if echo "$LIST_PROJECTS_RESPONSE" | grep -q '"code":200'; then
    test_pass "获取项目列表成功"
else
    test_fail "获取项目列表" "返回非200状态码"
fi

# 测试 1.3: 获取项目详情
if [ ! -z "$PROJECT_ID" ]; then
    echo ""
    echo "测试 1.3: 获取项目详情"
    echo "----------------------------------------"
    GET_PROJECT_RESPONSE=$(curl -s "${API_BASE}/projects/${PROJECT_ID}")
    echo "Response: $GET_PROJECT_RESPONSE"

    if echo "$GET_PROJECT_RESPONSE" | grep -q '"code":200'; then
        test_pass "获取项目详情成功"
    else
        test_fail "获取项目详情" "返回非200状态码"
    fi
fi

# 测试 1.4: 更新项目
if [ ! -z "$PROJECT_ID" ]; then
    echo ""
    echo "测试 1.4: 更新项目"
    echo "----------------------------------------"
    UPDATE_PROJECT_DATA='{
      "name": "更新后的测试项目",
      "description": "更新后的描述信息"
    }'

    UPDATE_PROJECT_RESPONSE=$(curl -s -X PUT "${API_BASE}/projects/${PROJECT_ID}" \
      -H "Content-Type: application/json" \
      -d "$UPDATE_PROJECT_DATA")
    echo "Response: $UPDATE_PROJECT_RESPONSE"

    if echo "$UPDATE_PROJECT_RESPONSE" | grep -q '"code":200'; then
        test_pass "更新项目成功"
    else
        test_fail "更新项目" "返回非200状态码"
    fi
fi

# ============================================
# 第二部分：数据集管理 API 测试
# ============================================
echo ""
echo "第二部分：数据集管理 API 测试"
echo "=========================================="

# 测试 2.1: 创建数据集（JSON 方式）
if [ ! -z "$PROJECT_ID" ]; then
    echo ""
    echo "测试 2.1: 创建数据集（JSON方式）"
    echo "----------------------------------------"
    DATASET_DATA='{
      "project_id": "'"$PROJECT_ID"'",
      "name": "测试种子数据集",
      "description": "包含100条测试问答对",
      "source_type": "upload",
      "file_path": "/mnt/shared/distill/test/seed_data.jsonl",
      "record_count": 100
    }'

    CREATE_DATASET_RESPONSE=$(curl -s -X POST "${API_BASE}/datasets" \
      -H "Content-Type: application/json" \
      -d "$DATASET_DATA")
    echo "Response: $CREATE_DATASET_RESPONSE"

    DATASET_ID=$(echo "$CREATE_DATASET_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ ! -z "$DATASET_ID" ]; then
        test_pass "创建数据集成功，ID: $DATASET_ID"
    else
        test_fail "创建数据集" "未能获取数据集ID"
    fi
fi

# 测试 2.2: 创建数据集（文件上传方式）
if [ ! -z "$PROJECT_ID" ]; then
    echo ""
    echo "测试 2.2: 创建数据集（文件上传方式）"
    echo "----------------------------------------"

    # 创建测试数据文件
    TEST_FILE="/tmp/test_dataset.jsonl"
    cat > "$TEST_FILE" << 'EOF'
{"instruction": "什么是机器学习？", "input": "", "output": "机器学习是人工智能的一个分支，它使计算机系统能够从数据中学习并改进，而无需明确编程。"}
{"instruction": "解释什么是深度学习", "input": "", "output": "深度学习是机器学习的一个子集，使用多层神经网络来学习数据的表示。"}
{"instruction": "Python 如何定义函数？", "input": "", "output": "在 Python 中使用 def 关键字定义函数，例如：def my_function(): pass"}
EOF

    UPLOAD_DATASET_RESPONSE=$(curl -s -X POST "${API_BASE}/projects/${PROJECT_ID}/datasets" \
      -F "file=@${TEST_FILE}" \
      -F "name=上传的测试数据集" \
      -F "description=通过文件上传的测试数据")
    echo "Response: $UPLOAD_DATASET_RESPONSE"

    UPLOAD_DATASET_ID=$(echo "$UPLOAD_DATASET_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ ! -z "$UPLOAD_DATASET_ID" ]; then
        test_pass "文件上传创建数据集成功，ID: $UPLOAD_DATASET_ID"
        # 如果第一个数据集创建失败，使用上传的数据集ID
        if [ -z "$DATASET_ID" ]; then
            DATASET_ID="$UPLOAD_DATASET_ID"
        fi
    else
        test_fail "文件上传创建数据集" "未能获取数据集ID"
    fi

    # 清理测试文件
    rm -f "$TEST_FILE"
fi

# 测试 2.3: 获取数据集列表
if [ ! -z "$PROJECT_ID" ]; then
    echo ""
    echo "测试 2.3: 获取数据集列表"
    echo "----------------------------------------"
    LIST_DATASETS_RESPONSE=$(curl -s "${API_BASE}/datasets?project_id=${PROJECT_ID}&page=1&page_size=10")
    echo "Response: $LIST_DATASETS_RESPONSE"

    if echo "$LIST_DATASETS_RESPONSE" | grep -q '"code":200'; then
        test_pass "获取数据集列表成功"
    else
        test_fail "获取数据集列表" "返回非200状态码"
    fi
fi

# 测试 2.4: 获取数据集详情
if [ ! -z "$DATASET_ID" ]; then
    echo ""
    echo "测试 2.4: 获取数据集详情"
    echo "----------------------------------------"
    GET_DATASET_RESPONSE=$(curl -s "${API_BASE}/datasets/${DATASET_ID}")
    echo "Response: $GET_DATASET_RESPONSE"

    if echo "$GET_DATASET_RESPONSE" | grep -q '"code":200'; then
        test_pass "获取数据集详情成功"
    else
        test_fail "获取数据集详情" "返回非200状态码"
    fi
fi

# ============================================
# 第三部分：流水线管理 API 测试
# ============================================
echo ""
echo "第三部分：流水线管理 API 测试"
echo "=========================================="

# 测试 3.1: 创建流水线
if [ ! -z "$PROJECT_ID" ] && [ ! -z "$DATASET_ID" ]; then
    echo ""
    echo "测试 3.1: 创建流水线"
    echo "----------------------------------------"
    PIPELINE_DATA='{
      "project_id": "'"$PROJECT_ID"'",
      "dataset_id": "'"$DATASET_ID"'",
      "trigger_mode": "manual",
      "training_config": {
        "num_train_epochs": 3,
        "per_device_train_batch_size": 4,
        "gradient_accumulation_steps": 4,
        "learning_rate": 0.00005,
        "weight_decay": 0.01,
        "warmup_ratio": 0.1,
        "lr_scheduler_type": "cosine",
        "save_steps": 500,
        "logging_steps": 10,
        "max_length": 2048,
        "lora_config": {
          "enabled": true,
          "r": 8,
          "alpha": 16,
          "dropout": 0.05,
          "target_modules": ["q_proj", "v_proj"]
        }
      },
      "resource_request": {
        "gpu_count": 1,
        "gpu_type": "A100",
        "memory_gb": 32,
        "cpu_cores": 8
      }
    }'

    CREATE_PIPELINE_RESPONSE=$(curl -s -X POST "${API_BASE}/pipelines" \
      -H "Content-Type: application/json" \
      -d "$PIPELINE_DATA")
    echo "Response: $CREATE_PIPELINE_RESPONSE"

    PIPELINE_ID=$(echo "$CREATE_PIPELINE_RESPONSE" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)

    if [ ! -z "$PIPELINE_ID" ]; then
        test_pass "创建流水线成功，ID: $PIPELINE_ID"
    else
        test_fail "创建流水线" "未能获取流水线ID"
    fi
fi

# 测试 3.2: 获取流水线列表
if [ ! -z "$PROJECT_ID" ]; then
    echo ""
    echo "测试 3.2: 获取流水线列表"
    echo "----------------------------------------"
    LIST_PIPELINES_RESPONSE=$(curl -s "${API_BASE}/pipelines?project_id=${PROJECT_ID}&page=1&page_size=10")
    echo "Response: $LIST_PIPELINES_RESPONSE"

    if echo "$LIST_PIPELINES_RESPONSE" | grep -q '"code":200'; then
        test_pass "获取流水线列表成功"
    else
        test_fail "获取流水线列表" "返回非200状态码"
    fi
fi

# 测试 3.3: 获取流水线详情
if [ ! -z "$PIPELINE_ID" ]; then
    echo ""
    echo "测试 3.3: 获取流水线详情"
    echo "----------------------------------------"
    GET_PIPELINE_RESPONSE=$(curl -s "${API_BASE}/pipelines/${PIPELINE_ID}")
    echo "Response: $GET_PIPELINE_RESPONSE"

    if echo "$GET_PIPELINE_RESPONSE" | grep -q '"code":200'; then
        test_pass "获取流水线详情成功"
    else
        test_fail "获取流水线详情" "返回非200状态码"
    fi
fi

# 测试 3.4: 获取流水线阶段列表
if [ ! -z "$PIPELINE_ID" ]; then
    echo ""
    echo "测试 3.4: 获取流水线阶段列表"
    echo "----------------------------------------"
    LIST_STAGES_RESPONSE=$(curl -s "${API_BASE}/pipelines/${PIPELINE_ID}/stages")
    echo "Response: $LIST_STAGES_RESPONSE"

    if echo "$LIST_STAGES_RESPONSE" | grep -q '"code":200'; then
        # 检查是否创建了6个阶段
        STAGE_COUNT=$(echo "$LIST_STAGES_RESPONSE" | grep -o '"stage_order"' | wc -l)
        if [ "$STAGE_COUNT" -eq 6 ]; then
            test_pass "获取流水线阶段列表成功，共${STAGE_COUNT}个阶段"
        else
            test_fail "获取流水线阶段列表" "阶段数量不正确，期望6个，实际${STAGE_COUNT}个"
        fi
    else
        test_fail "获取流水线阶段列表" "返回非200状态码"
    fi
fi

# 测试 3.5: 启动流水线
if [ ! -z "$PIPELINE_ID" ]; then
    echo ""
    echo "测试 3.5: 启动流水线"
    echo "----------------------------------------"
    test_info "注意：启动流水线会触发实际的Docker容器执行，可能需要较长时间"

    START_PIPELINE_RESPONSE=$(curl -s -X POST "${API_BASE}/pipelines/${PIPELINE_ID}/start")
    echo "Response: $START_PIPELINE_RESPONSE"

    if echo "$START_PIPELINE_RESPONSE" | grep -q '"code":200'; then
        test_pass "启动流水线成功"

        # 等待几秒钟，让流水线开始执行
        sleep 3

        # 再次获取流水线状态，确认状态已改变
        PIPELINE_STATUS_RESPONSE=$(curl -s "${API_BASE}/pipelines/${PIPELINE_ID}")
        echo "Pipeline status after start: $PIPELINE_STATUS_RESPONSE"

        if echo "$PIPELINE_STATUS_RESPONSE" | grep -q -E '"status":"(scheduled|preparing|running)"'; then
            test_pass "流水线状态已更新为执行中"
        else
            test_fail "流水线状态检查" "状态未更新为执行状态"
        fi
    else
        test_fail "启动流水线" "返回非200状态码"
    fi
fi

# 测试 3.6: 取消流水线
if [ ! -z "$PIPELINE_ID" ]; then
    echo ""
    echo "测试 3.6: 取消流水线"
    echo "----------------------------------------"

    CANCEL_PIPELINE_RESPONSE=$(curl -s -X POST "${API_BASE}/pipelines/${PIPELINE_ID}/cancel")
    echo "Response: $CANCEL_PIPELINE_RESPONSE"

    if echo "$CANCEL_PIPELINE_RESPONSE" | grep -q '"code":200'; then
        test_pass "取消流水线成功"
    else
        # 如果流水线已经完成或失败，取消可能会失败，这是正常的
        test_info "取消流水线请求已发送（流水线可能已完成）"
    fi
fi

# ============================================
# 第四部分：资源管理 API 测试
# ============================================
echo ""
echo "第四部分：资源管理 API 测试"
echo "=========================================="

# 测试 4.1: 获取节点列表
echo ""
echo "测试 4.1: 获取节点列表"
echo "----------------------------------------"
LIST_NODES_RESPONSE=$(curl -s "${API_BASE}/resources/nodes")
echo "Response: $LIST_NODES_RESPONSE"

if echo "$LIST_NODES_RESPONSE" | grep -q '"code":200'; then
    test_pass "获取节点列表成功"

    # 检查是否有worker节点
    if echo "$LIST_NODES_RESPONSE" | grep -q '"node_name"'; then
        NODE_COUNT=$(echo "$LIST_NODES_RESPONSE" | grep -o '"node_name"' | wc -l)
        test_info "检测到 ${NODE_COUNT} 个 Worker 节点"
    else
        test_info "当前没有在线的 Worker 节点"
    fi
else
    test_fail "获取节点列表" "返回非200状态码"
fi

# 测试 4.2: 获取节点详情（如果有节点的话）
NODE_NAME=$(echo "$LIST_NODES_RESPONSE" | grep -o '"node_name":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ ! -z "$NODE_NAME" ]; then
    echo ""
    echo "测试 4.2: 获取节点详情"
    echo "----------------------------------------"
    GET_NODE_RESPONSE=$(curl -s "${API_BASE}/resources/nodes/${NODE_NAME}")
    echo "Response: $GET_NODE_RESPONSE"

    if echo "$GET_NODE_RESPONSE" | grep -q '"code":200'; then
        test_pass "获取节点详情成功"
    else
        test_fail "获取节点详情" "返回非200状态码"
    fi
fi

# ============================================
# 测试总结
# ============================================
echo ""
echo "=========================================="
echo "测试总结"
echo "=========================================="
echo -e "总测试数: ${TOTAL_TESTS}"
echo -e "${GREEN}通过: ${PASSED_TESTS}${NC}"
echo -e "${RED}失败: ${FAILED_TESTS}${NC}"
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "${GREEN}🎉 所有测试通过！${NC}"
    echo ""
    echo "测试结果保存的数据："
    [ ! -z "$PROJECT_ID" ] && echo "  - 项目ID: $PROJECT_ID"
    [ ! -z "$DATASET_ID" ] && echo "  - 数据集ID: $DATASET_ID"
    [ ! -z "$PIPELINE_ID" ] && echo "  - 流水线ID: $PIPELINE_ID"
    echo ""
    echo "后续可以使用这些ID进行进一步的测试和验证"
    exit 0
else
    echo -e "${RED}❌ 有 ${FAILED_TESTS} 个测试失败${NC}"
    echo ""
    echo "请检查："
    echo "  1. Docker Compose 服务是否正常运行"
    echo "  2. 数据库是否正确初始化"
    echo "  3. 服务日志中是否有错误信息"
    echo ""
    echo "查看服务日志："
    echo "  docker-compose logs gcs-server"
    echo "  docker-compose logs gcs-worker-1"
    exit 1
fi
