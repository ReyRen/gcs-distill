# GCS-Distill API 参考文档

## 基础信息

- **Base URL**: `http://172.18.36.230:18080/api/v1`
- **Content-Type**: `application/json`
- **响应格式**: JSON

所有响应遵循以下格式：

```json
{
  "code": 200,
  "message": "操作成功",
  "data": {}
}
```

`data` 可能是对象、数组，部分操作型接口仅返回 `code` 和 `message`。

## 错误码

- `200` - 成功
- `400` - 请求参数错误
- `404` - 资源不存在
- `500` - 服务器内部错误

---

## 项目管理 API

### 1. 创建项目

**POST** `/api/v1/projects`

**请求体**:
```json
{
  "name": "我的蒸馏项目",
  "description": "项目描述",
  "teacher_model_config": {
    "model_name": "gpt-4",
    "provider_type": "api",
    "base_url": "https://api.openai.com/v1",
    "api_key": "sk-xxx",
    "temperature": 0.7
  },
  "student_model_config": {
    "model_name": "qwen-7b",
    "provider_type": "local",
    "model_path": "/mnt/shared/distill/models/qwen-7b"
  }
}
```

**字段说明**:
- `teacher_model_config`: 教师模型配置
  - `provider_type`: 可以是 `api` (API调用) 或 `local` (本地模型)
  - `model_name`: 模型名称
  - `endpoint`, `api_secret_ref`: API 模型所需字段
- `student_model_config`: 学生模型配置
  - `provider_type`: **必须为 `local`** (系统运行在离线环境，不支持在线模型)
  - `model_name`: 模型名称
  - `model_path`: **必填** - 本地模型文件路径，如 `/mnt/shared/distill/models/qwen-7b`

**响应**:
```json
{
  "code": 200,
  "message": "项目创建成功",
  "data": {
    "id": "uuid-xxx",
    "name": "我的蒸馏项目",
    "created_at": "2026-04-13T10:00:00Z",
    ...
  }
}
```

### 2. 获取项目列表

**GET** `/api/v1/projects?page=1&page_size=20`

**查询参数**:
- `page`: 页码（默认 1）
- `page_size`: 每页大小（默认 20，最大 100）

**响应**:
```json
{
  "code": 200,
  "message": "获取项目列表成功",
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

### 3. 获取项目详情

**GET** `/api/v1/projects/{id}`

### 4. 更新项目

**PUT** `/api/v1/projects/{id}`

### 5. 删除项目

**DELETE** `/api/v1/projects/{id}`

---

## 数据集管理 API

### 1. 创建数据集

**POST** `/api/v1/datasets`

**请求体**:
```json
{
  "project_id": "uuid-xxx",
  "name": "训练数据集",
  "description": "数据集描述",
  "source_type": "upload",
  "file_format": "jsonl",
  "record_count": 10000
}
```

**source_type 可选值**:
- `upload` - 用户上传
- `import` - 从外部导入
- `generated` - 自动生成

### 2. 获取数据集列表

**GET** `/api/v1/datasets?project_id=xxx&page=1&page_size=20`

**查询参数**:
- `project_id`: 项目 ID（必填）
- `page`: 页码
- `page_size`: 每页大小

**响应**:
```json
{
  "code": 200,
  "message": "获取数据集列表成功",
  "data": {
    "items": [...],
    "total": 10,
    "page": 1,
    "page_size": 20
  }
}
```

### 3. 获取数据集详情

**GET** `/api/v1/datasets/{id}`

### 4. 更新数据集

**PUT** `/api/v1/datasets/{id}`

当前实现仅更新 `name`、`description`、`record_count`。前端至少需要传 `name`。

### 5. 删除数据集

**DELETE** `/api/v1/datasets/{id}`

---

## 流水线管理 API

### 1. 创建流水线

**POST** `/api/v1/pipelines`

**请求体**:
```json
{
  "project_id": "uuid-xxx",
  "dataset_id": "uuid-yyy",
  "training_config": {
    "num_train_epochs": 3,
    "per_device_train_batch_size": 8,
    "learning_rate": 0.0001,
    "warmup_steps": 100
  },
  "resource_request": {
    "gpu_count": 2,
    "gpu_device_ids": "0,1",
    "memory_gb": 32,
    "cpu_cores": 8
  }
}
```

**resource_request 字段说明**:
- `gpu_count`: GPU 数量（必填）
  - 设置为 `-1` 表示使用所有可用 GPU
  - 设置为正整数表示使用指定数量的 GPU
- `gpu_device_ids`: GPU 设备 ID（可选）
  - 格式：`"0,1,2"` 表示使用 GPU 0、1 和 2
  - 如果同时指定了 `gpu_count` 和 `gpu_device_ids`，优先使用 `gpu_device_ids`
  - 留空则由系统自动分配
- `memory_gb`: 内存大小（GB）（可选）
- `cpu_cores`: CPU 核心数（可选）

**响应**:
```json
{
  "code": 200,
  "message": "流水线创建成功",
  "data": {
    "id": "uuid-zzz",
    "status": "pending",
    "current_stage": 0,
    "created_at": "2026-04-13T10:00:00Z"
  }
}
```

### 2. 获取流水线列表

**GET** `/api/v1/pipelines?project_id=xxx&page=1&page_size=20`

**查询参数**:
- `project_id`: 项目 ID（必填）
- `page`: 页码
- `page_size`: 每页大小

**响应**:
```json
{
  "code": 200,
  "message": "获取流水线列表成功",
  "data": {
    "items": [...],
    "total": 10,
    "page": 1,
    "page_size": 20
  }
}
```

### 3. 获取流水线详情

**GET** `/api/v1/pipelines/{id}`

**响应**:
```json
{
  "code": 200,
  "message": "获取流水线成功",
  "data": {
    "id": "uuid-zzz",
    "project_id": "uuid-xxx",
    "dataset_id": "uuid-yyy",
    "status": "running",
    "current_stage": 3,
    "started_at": "2026-04-13T10:01:00Z",
    "training_config": {...},
    "resource_request": {...}
  }
}
```

**流水线状态**:
- `pending` - 等待中
- `running` - 运行中
- `succeeded` - 成功
- `failed` - 失败
- `canceled` - 已取消

### 4. 启动流水线

**POST** `/api/v1/pipelines/{id}/start`

启动一个处于 `pending` 状态的流水线。

**响应**:
```json
{
  "code": 200,
  "message": "流水线启动成功"
}
```

### 5. 取消流水线

**POST** `/api/v1/pipelines/{id}/cancel`

取消处于 `pending` 或 `running` 状态的流水线。

**响应**:
```json
{
  "code": 200,
  "message": "流水线取消成功"
}
```

### 6. 获取流水线阶段列表

**GET** `/api/v1/pipelines/{id}/stages`

**响应**:
```json
{
  "code": 200,
  "message": "获取阶段列表成功",
  "data": [
    {
      "id": "stage-1",
      "pipeline_run_id": "uuid-zzz",
      "stage_type": "teacher_config",
      "stage_order": 1,
      "status": "succeeded",
      "started_at": "2026-04-13T10:01:00Z",
      "finished_at": "2026-04-13T10:02:00Z"
    },
    {
      "id": "stage-2",
      "stage_type": "dataset_build",
      "stage_order": 2,
      "status": "running",
      "started_at": "2026-04-13T10:02:00Z"
    },
    ...
  ]
}
```

**阶段类型（按顺序）**:
1. `teacher_config` - 教师模型配置
2. `dataset_build` - 数据集构建
3. `teacher_infer` - 教师模型推理
4. `data_govern` - 数据治理
5. `student_train` - 学生模型训练
6. `evaluate` - 评估

**阶段状态**:
- `pending` - 等待中
- `running` - 运行中
- `succeeded` - 成功
- `failed` - 失败
- `canceled` - 已取消

### 7. 获取阶段日志（完整）

**GET** `/api/v1/pipelines/{id}/stages/{stage_id}/logs`

获取指定阶段的完整日志内容。

**响应**:
```json
{
  "code": 200,
  "message": "获取日志成功",
  "data": {
    "logs": "日志内容...",
    "log_path": "/path/to/log/file",
    "stage_id": "stage-uuid",
    "stage_type": "teacher_infer"
  }
}
```

### 8. 获取阶段实时日志

**GET** `/api/v1/pipelines/{id}/stages/{stage_id}/logs/stream?tail=100`

获取指定阶段的实时日志（最后N行）。

**查询参数**:
- `tail`: 返回最后N行日志（默认 100）

**响应**:
```json
{
  "code": 200,
  "message": "获取实时日志成功",
  "data": {
    "logs": "最近的日志内容...",
    "log_path": "/path/to/log/file",
    "stage_id": "stage-uuid",
    "stage_type": "teacher_infer",
    "status": "running"
  }
}
```

### 9. 下载阶段日志

**GET** `/api/v1/pipelines/{id}/stages/{stage_id}/logs/download`

下载指定阶段的日志文件。

**响应**: 直接返回日志文件，浏览器会提示下载。

---

## 资源管理 API

### 1. 获取节点列表

**GET** `/api/v1/resources/nodes`

**响应**:
```json
{
  "code": 200,
  "message": "获取节点列表成功",
  "data": [
    {
      "node_name": "worker-1",
      "node_addr": "192.168.1.10:50051",
      "status": "online",
      "total_gpu": 4,
      "available_gpu": 2,
      "total_memory_gb": 128,
      "total_cpu": 32,
      "last_heartbeat": "2026-04-13T10:05:00Z"
    },
    ...
  ]
}
```

**节点状态**:
- `online` - 在线
- `busy` - 繁忙（所有 GPU 已分配）
- `offline` - 离线

### 2. 获取节点详情

**GET** `/api/v1/resources/nodes/{name}`

---

## 健康检查

### Health Check

**GET** `/health`

**响应**:
```json
{
  "status": "ok"
}
```

---

## 使用示例

### 完整流程示例

#### 1. 创建项目

```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试项目",
    "description": "测试描述",
    "teacher_model_config": {
      "model_name": "gpt-4",
      "provider_type": "api",
      "base_url": "https://api.openai.com/v1",
      "api_key": "sk-xxx"
    },
    "student_model_config": {
      "model_name": "qwen-7b",
      "provider_type": "local",
      "model_path": "/mnt/shared/distill/models/qwen-7b"
    }
  }'
```

#### 2. 创建数据集

```bash
curl -X POST http://172.18.36.230:18080/api/v1/datasets \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "项目ID",
    "name": "训练数据",
    "source_type": "upload",
    "file_format": "jsonl"
  }'
```

#### 3. 创建并启动流水线

```bash
# 创建流水线
curl -X POST http://172.18.36.230:18080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "项目ID",
    "dataset_id": "数据集ID",
    "training_config": {
      "num_train_epochs": 3,
      "learning_rate": 0.0001
    },
    "resource_request": {
      "gpu_count": 1
    }
  }'

# 启动流水线
curl -X POST http://172.18.36.230:18080/api/v1/pipelines/{流水线ID}/start
```

#### 4. 查询流水线状态

```bash
# 获取流水线详情
curl http://172.18.36.230:18080/api/v1/pipelines/{流水线ID}

# 获取阶段列表
curl http://172.18.36.230:18080/api/v1/pipelines/{流水线ID}/stages
```

#### 5. 查看可用节点

```bash
curl http://172.18.36.230:18080/api/v1/resources/nodes
```

---

## 注意事项

1. 所有时间戳使用 RFC3339 格式 (ISO 8601)
2. 分页查询默认返回 20 条记录，最大支持 100 条
3. UUID 格式的 ID 由服务器自动生成
4. 删除项目会级联删除相关的数据集和流水线
5. 流水线创建后会自动创建六个阶段，按顺序执行
6. 节点信息存储在 Redis 中，TTL 为 5 分钟
7. Worker 节点需要定期发送心跳，否则会被标记为离线

---

## 错误响应示例

```json
{
  "code": 400,
  "message": "请求参数格式错误: 项目名称不能为空"
}
```

```json
{
  "code": 404,
  "message": "项目不存在: uuid-xxx"
}
```

```json
{
  "code": 500,
  "message": "服务器内部错误"
}
```
