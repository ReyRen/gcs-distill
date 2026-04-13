# GCS-Distill API 测试指南

## 概述

本文档提供了 GCS-Distill 平台 API 的完整测试指南，包括自动化测试脚本使用说明和手动测试步骤。

## 快速开始

### 1. 启动服务

```bash
# 进入项目目录
cd gcs-distill

# 使用 Docker Compose 启动所有服务
docker-compose up -d

# 等待服务启动（约30秒）
docker-compose ps

# 查看服务日志
docker-compose logs -f gcs-server
```

### 2. 运行自动化测试

```bash
# 运行完整的API测试套件
./test-apis.sh

# 或使用 make 命令
make test-api
```

### 3. 验证服务状态

```bash
# 健康检查
curl http://localhost:18080/health

# 应返回: {"status":"ok"}
```

## 测试脚本说明

`test-apis.sh` 是一个全面的 API 测试脚本，测试所有核心接口功能。

### 测试覆盖范围

#### 第一部分：项目管理 API（5个接口）
- ✅ 创建项目 - `POST /api/v1/projects`
- ✅ 获取项目列表 - `GET /api/v1/projects`
- ✅ 获取项目详情 - `GET /api/v1/projects/{id}`
- ✅ 更新项目 - `PUT /api/v1/projects/{id}`
- ✅ 删除项目 - `DELETE /api/v1/projects/{id}`（脚本中未包含，避免误删）

#### 第二部分：数据集管理 API（5个接口）
- ✅ 创建数据集（JSON） - `POST /api/v1/datasets`
- ✅ 创建数据集（文件上传） - `POST /api/v1/projects/{id}/datasets`
- ✅ 获取数据集列表 - `GET /api/v1/datasets`
- ✅ 获取数据集详情 - `GET /api/v1/datasets/{id}`
- ⏸️ 更新数据集 - `PUT /api/v1/datasets/{id}`（保留接口）
- ⏸️ 删除数据集 - `DELETE /api/v1/datasets/{id}`（保留接口）

#### 第三部分：流水线管理 API（6个接口）
- ✅ 创建流水线 - `POST /api/v1/pipelines`
- ✅ 获取流水线列表 - `GET /api/v1/pipelines`
- ✅ 获取流水线详情 - `GET /api/v1/pipelines/{id}`
- ✅ 获取流水线阶段 - `GET /api/v1/pipelines/{id}/stages`
- ✅ 启动流水线 - `POST /api/v1/pipelines/{id}/start`
- ✅ 取消流水线 - `POST /api/v1/pipelines/{id}/cancel`

#### 第四部分：资源管理 API（2个接口）
- ✅ 获取节点列表 - `GET /api/v1/resources/nodes`
- ✅ 获取节点详情 - `GET /api/v1/resources/nodes/{name}`

### 测试输出示例

```
==========================================
GCS-Distill API 综合测试
==========================================

Step 0: 检查服务健康状态...
----------------------------------------
✓ PASS - 服务健康检查

第一部分：项目管理 API 测试
==========================================
测试 1.1: 创建项目
----------------------------------------
✓ PASS - 创建项目成功，ID: abc123...

测试 1.2: 获取项目列表
----------------------------------------
✓ PASS - 获取项目列表成功

...

==========================================
测试总结
==========================================
总测试数: 18
通过: 18
失败: 0

🎉 所有测试通过！
```

## 手动测试步骤

### 场景一：完整蒸馏流程测试

#### 步骤 1：创建项目

```bash
curl -X POST http://localhost:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "客服问答蒸馏",
    "description": "将 Qwen2.5-7B 蒸馏到 Qwen2.5-0.5B",
    "business_scenario": "智能客服",
    "teacher_model_config": {
      "provider_type": "local",
      "model_name": "Qwen/Qwen2.5-7B-Instruct",
      "temperature": 0.7,
      "max_tokens": 2048
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
```

**期望响应：**
```json
{
  "code": 200,
  "message": "项目创建成功",
  "data": {
    "id": "项目ID",
    "name": "客服问答蒸馏",
    ...
  }
}
```

#### 步骤 2：上传数据集

创建测试数据文件 `seed_data.jsonl`:

```jsonl
{"instruction": "如何重置密码？", "input": "", "output": "您可以点击登录页面的'忘记密码'链接，然后按照提示操作。"}
{"instruction": "订单如何退款？", "input": "", "output": "请在订单详情页点击'申请退款'按钮，填写退款原因后提交。"}
{"instruction": "物流信息在哪里查看？", "input": "", "output": "您可以在'我的订单'中点击订单编号，查看物流跟踪信息。"}
```

上传文件：

```bash
curl -X POST http://localhost:18080/api/v1/projects/{项目ID}/datasets \
  -F "file=@seed_data.jsonl" \
  -F "name=客服问答种子数据" \
  -F "description=包含100条客服问答种子数据"
```

**期望响应：**
```json
{
  "code": 200,
  "message": "数据集上传成功",
  "data": {
    "id": "数据集ID",
    "project_id": "项目ID",
    "name": "客服问答种子数据",
    ...
  }
}
```

#### 步骤 3：创建流水线

```bash
curl -X POST http://localhost:18080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "项目ID",
    "dataset_id": "数据集ID",
    "trigger_mode": "manual",
    "training_config": {
      "num_train_epochs": 3,
      "per_device_train_batch_size": 4,
      "learning_rate": 0.00005,
      "warmup_ratio": 0.1,
      "lr_scheduler_type": "cosine",
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
      "memory_gb": 32,
      "cpu_cores": 8
    }
  }'
```

**期望响应：**
```json
{
  "code": 200,
  "message": "流水线创建成功",
  "data": {
    "id": "流水线ID",
    "status": "pending",
    "current_stage": 0,
    ...
  }
}
```

#### 步骤 4：查看流水线阶段

```bash
curl http://localhost:18080/api/v1/pipelines/{流水线ID}/stages
```

**期望响应：**
```json
{
  "code": 200,
  "message": "获取阶段列表成功",
  "data": [
    {
      "id": "阶段1-ID",
      "stage_type": "teacher_config",
      "stage_order": 1,
      "status": "pending"
    },
    {
      "id": "阶段2-ID",
      "stage_type": "dataset_build",
      "stage_order": 2,
      "status": "pending"
    },
    ...共6个阶段
  ]
}
```

#### 步骤 5：启动流水线

```bash
curl -X POST http://localhost:18080/api/v1/pipelines/{流水线ID}/start
```

**期望响应：**
```json
{
  "code": 200,
  "message": "流水线启动成功"
}
```

#### 步骤 6：监控流水线执行

```bash
# 每隔几秒查询一次流水线状态
watch -n 3 "curl -s http://localhost:18080/api/v1/pipelines/{流水线ID} | jq"

# 或查看阶段执行情况
watch -n 3 "curl -s http://localhost:18080/api/v1/pipelines/{流水线ID}/stages | jq"
```

#### 步骤 7：查看 Worker 节点状态

```bash
curl http://localhost:18080/api/v1/resources/nodes
```

**期望响应：**
```json
{
  "code": 200,
  "message": "获取节点列表成功",
  "data": [
    {
      "node_name": "worker-1",
      "node_addr": "gcs-worker-1:50052",
      "status": "online",
      "total_gpu": 4,
      "available_gpu": 2,
      "total_memory_gb": 128,
      "total_cpu": 32,
      "last_heartbeat": "2026-04-13T12:00:00Z"
    }
  ]
}
```

### 场景二：数据集管理测试

#### 测试文件上传功能

```bash
# 创建测试数据
cat > test_upload.jsonl << EOF
{"instruction": "测试问题1", "input": "", "output": "测试答案1"}
{"instruction": "测试问题2", "input": "", "output": "测试答案2"}
EOF

# 上传文件
curl -X POST http://localhost:18080/api/v1/projects/{项目ID}/datasets \
  -F "file=@test_upload.jsonl" \
  -F "name=测试上传数据集" \
  -F "description=测试文件上传功能"

# 验证上传成功
curl http://localhost:18080/api/v1/datasets?project_id={项目ID}
```

### 场景三：错误处理测试

#### 测试无效的项目ID

```bash
curl http://localhost:18080/api/v1/projects/invalid-id

# 期望返回 404 错误
{
  "code": 404,
  "message": "项目不存在: invalid-id"
}
```

#### 测试缺少必填字段

```bash
curl -X POST http://localhost:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "description": "缺少name字段"
  }'

# 期望返回 400 错误
{
  "code": 400,
  "message": "请求参数格式错误: ..."
}
```

## 常见问题排查

### 问题 1：服务健康检查失败

**症状：** `curl http://localhost:18080/health` 无响应或超时

**解决方法：**
```bash
# 检查容器状态
docker-compose ps

# 查看服务日志
docker-compose logs gcs-server

# 重启服务
docker-compose restart gcs-server
```

### 问题 2：数据库连接错误

**症状：** 接口返回 500 错误，日志显示数据库连接失败

**解决方法：**
```bash
# 检查 PostgreSQL 容器
docker-compose ps postgres

# 查看 PostgreSQL 日志
docker-compose logs postgres

# 重新初始化数据库
docker-compose down -v
docker-compose up -d
```

### 问题 3：Worker 节点不在线

**症状：** `/api/v1/resources/nodes` 返回空列表

**解决方法：**
```bash
# 检查 Worker 容器
docker-compose ps gcs-worker-1

# 查看 Worker 日志
docker-compose logs gcs-worker-1

# 检查 Redis 连接
docker-compose exec redis redis-cli
> KEYS worker:*
```

### 问题 4：流水线启动失败

**症状：** 流水线状态变为 `failed`

**解决方法：**
```bash
# 查看流水线详情，获取错误信息
curl http://localhost:18080/api/v1/pipelines/{流水线ID}

# 查看阶段详情
curl http://localhost:18080/api/v1/pipelines/{流水线ID}/stages

# 检查 Worker 日志
docker-compose logs gcs-worker-1

# 检查共享存储权限
ls -la /storage-md0/renyuan/gcs-distill-data/shared-workspace
```

## 性能测试

### 并发测试

使用 `ab` (Apache Bench) 进行简单的并发测试：

```bash
# 安装 ab
sudo apt-get install apache2-utils

# 测试健康检查接口（100个请求，并发10）
ab -n 100 -c 10 http://localhost:18080/health

# 测试项目列表接口
ab -n 100 -c 10 http://localhost:18080/api/v1/projects
```

### 压力测试

使用 `wrk` 进行更专业的压力测试：

```bash
# 安装 wrk
sudo apt-get install wrk

# 运行30秒压力测试，12个线程，400个连接
wrk -t12 -c400 -d30s http://localhost:18080/health
```

## 测试数据清理

### 清理所有测试数据

```bash
# 停止所有服务
docker-compose down

# 删除数据卷（谨慎操作！）
docker-compose down -v

# 重新启动
docker-compose up -d
```

### 清理特定数据

```bash
# 删除特定项目
curl -X DELETE http://localhost:18080/api/v1/projects/{项目ID}

# 删除特定数据集
curl -X DELETE http://localhost:18080/api/v1/datasets/{数据集ID}
```

## 自动化测试集成

### CI/CD 集成

在 CI/CD 流程中集成测试脚本：

```yaml
# .github/workflows/api-test.yml
name: API Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Start services
        run: docker-compose up -d

      - name: Wait for services
        run: sleep 30

      - name: Run API tests
        run: ./test-apis.sh

      - name: Cleanup
        run: docker-compose down -v
```

## 测试检查清单

在发布前，请确保以下测试全部通过：

- [ ] 服务健康检查
- [ ] 创建项目（含教师/学生模型配置）
- [ ] 获取项目列表（分页正常）
- [ ] 更新项目信息
- [ ] 上传数据集文件（支持 JSONL 格式）
- [ ] 获取数据集列表
- [ ] 创建流水线（自动创建6个阶段）
- [ ] 查看流水线阶段详情
- [ ] 启动流水线（状态变更正常）
- [ ] 取消流水线
- [ ] 查看 Worker 节点状态
- [ ] 错误处理（404、400、500）
- [ ] 数据验证（必填字段检查）
- [ ] 并发访问稳定性

## 测试报告模板

### 测试环境信息
- 操作系统: Ubuntu 22.04
- Docker 版本: 20.10.x
- Docker Compose 版本: 2.x
- 测试时间: 2026-04-13
- 测试人员: [姓名]

### 测试结果汇总
- 总测试用例数: 18
- 通过: 18
- 失败: 0
- 跳过: 0

### 详细测试结果
[附加测试脚本输出]

### 发现的问题
[列出测试中发现的任何问题]

### 建议改进
[列出改进建议]

---

**最后更新：** 2026-04-13
**文档版本：** 1.0
