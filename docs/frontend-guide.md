# GCS-Distill 前端实现指南

本文档为前端开发团队提供完整的 API 接口说明、数据结构定义和页面布局建议。

## 目录

- [系统概述](#系统概述)
- [API 接口文档](#api-接口文档)
- [数据结构定义](#数据结构定义)
- [页面布局建议](#页面布局建议)
- [状态管理建议](#状态管理建议)
- [开发示例](#开发示例)

---

## 系统概述

GCS-Distill 是一个大模型蒸馏平台，核心流程包含 6 个阶段：

1. **教师模型配置** - 配置教师大模型参数
2. **蒸馏数据构建** - 上传种子数据集
3. **教师推理** - 教师模型生成标注数据
4. **数据治理** - 清洗、过滤、去重
5. **学生训练** - 训练轻量级学生模型
6. **效果评估** - 评估蒸馏效果

### 核心概念

- **项目 (Project)**: 一个蒸馏项目，包含教师模型和学生模型配置
- **数据集 (Dataset)**: 种子数据或生成的蒸馏数据
- **流水线 (Pipeline Run)**: 一次完整的蒸馏执行流程
- **阶段 (Stage Run)**: 流水线中的某个阶段
- **Worker 节点**: 执行容器任务的计算节点

---

## API 接口文档

### Base URL

```
http://172.18.36.230:18080/api/v1
```

### 通用响应格式

所有接口统一返回格式：

```json
{
  "code": 200,
  "message": "成功消息",
  "data": { /* 具体数据，可能是对象或数组 */ }
}
```

错误响应：

```json
{
  "code": 400/404/500,
  "message": "错误消息"
}
```

---

### 1. 项目管理 API

#### 1.1 创建项目

**请求**

```http
POST /api/v1/projects
Content-Type: application/json
```

**请求体**

```json
{
  "name": "客服问答蒸馏",
  "description": "将 Qwen2.5-7B 蒸馏到 Qwen2.5-0.5B",
  "business_scenario": "智能客服",
  "teacher_model_config": {
    "provider_type": "api",
    "model_name": "Qwen/Qwen2.5-7B-Instruct",
    "endpoint": "https://api.openai.com/v1/chat/completions",
    "api_secret_ref": "openai_key",
    "temperature": 0.7,
    "max_tokens": 2048,
    "concurrency": 10,
    "timeout_seconds": 60
  },
  "student_model_config": {
    "provider_type": "local",
    "model_name": "Qwen/Qwen2.5-0.5B-Instruct"
  },
  "evaluation_config": {
    "metrics": ["bleu", "rouge", "accuracy"],
    "test_set_ratio": 0.2
  }
}
```

**响应**

```json
{
  "code": 200,
  "message": "项目创建成功",
  "data": {
    "id": "uuid-string",
    "name": "客服问答蒸馏",
    "description": "将 Qwen2.5-7B 蒸馏到 Qwen2.5-0.5B",
    "business_scenario": "智能客服",
    "teacher_model_config": { /* ... */ },
    "student_model_config": { /* ... */ },
    "evaluation_config": { /* ... */ },
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### 1.2 获取项目列表

**请求**

```http
GET /api/v1/projects?page=1&page_size=10
```

**响应**

```json
{
  "code": 200,
  "message": "获取项目列表成功",
  "data": {
    "items": [
      {
        "id": "uuid-1",
        "name": "客服问答蒸馏",
        "description": "...",
        "created_at": "2024-01-01T00:00:00Z",
        "updated_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

#### 1.3 获取项目详情

**请求**

```http
GET /api/v1/projects/{project_id}
```

**响应**

```json
{
  "code": 200,
  "message": "获取项目成功",
  "data": {
    "id": "uuid-string",
    "name": "客服问答蒸馏",
    /* 完整项目信息 */
  }
}
```

#### 1.4 更新项目

**请求**

```http
PUT /api/v1/projects/{project_id}
Content-Type: application/json
```

**请求体**（只需传要更新的字段）

```json
{
  "description": "更新后的描述",
  "teacher_model_config": {
    "temperature": 0.8
  }
}
```

#### 1.5 删除项目

**请求**

```http
DELETE /api/v1/projects/{project_id}
```

**响应**

```json
{
  "code": 200,
  "message": "项目删除成功"
}
```

---

### 2. 数据集管理 API

#### 2.1 创建数据集

**请求**

```http
POST /api/v1/datasets
Content-Type: application/json
```

**请求体**

```json
{
  "project_id": "project-uuid",
  "name": "种子数据集 v1",
  "description": "包含 1000 条客服问答种子数据",
  "source_type": "upload",
  "file_path": "/mnt/shared/distill/project-uuid/seeds.jsonl",
  "record_count": 1000
}
```

**响应**

```json
{
  "code": 200,
  "message": "数据集创建成功",
  "data": {
    "id": "dataset-uuid",
    "project_id": "project-uuid",
    "name": "种子数据集 v1",
    "description": "包含 1000 条客服问答种子数据",
    "source_type": "upload",
    "file_path": "/mnt/shared/distill/project-uuid/seeds.jsonl",
    "record_count": 1000,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### 2.2 获取数据集列表

**请求**

```http
GET /api/v1/datasets?project_id={project_id}&page=1&page_size=10
```

**响应**

```json
{
  "code": 200,
  "message": "获取数据集列表成功",
  "data": {
    "items": [
      {
        "id": "dataset-uuid",
        "project_id": "project-uuid",
        "name": "种子数据集 v1",
        "source_type": "upload",
        "record_count": 1000,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

#### 2.3 获取数据集详情

**请求**

```http
GET /api/v1/datasets/{dataset_id}
```

#### 2.4 更新数据集

**请求**

```http
PUT /api/v1/datasets/{dataset_id}
Content-Type: application/json
```

#### 2.5 删除数据集

**请求**

```http
DELETE /api/v1/datasets/{dataset_id}
```

---

### 3. 流水线管理 API

#### 3.1 创建流水线

**请求**

```http
POST /api/v1/pipelines
Content-Type: application/json
```

**请求体**

```json
{
  "project_id": "project-uuid",
  "dataset_id": "dataset-uuid",
  "trigger_mode": "manual",
  "training_config": {
    "num_train_epochs": 3,
    "per_device_train_batch_size": 4,
    "gradient_accumulation_steps": 4,
    "learning_rate": 5e-5,
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
    "gpu_count": 2,
    "gpu_device_ids": "0,1",
    "gpu_type": "A100",
    "memory_gb": 32,
    "cpu_cores": 8
  }
}
```

**响应**

```json
{
  "code": 200,
  "message": "流水线创建成功",
  "data": {
    "id": "pipeline-uuid",
    "project_id": "project-uuid",
    "dataset_id": "dataset-uuid",
    "status": "pending",
    "current_stage": 0,
    "trigger_mode": "manual",
    "training_config": { /* ... */ },
    "resource_request": { /* ... */ },
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

#### 3.2 启动流水线

**请求**

```http
POST /api/v1/pipelines/{pipeline_id}/start
```

**响应**

```json
{
  "code": 200,
  "message": "流水线启动成功"
}
```

**重要说明**：

流水线启动是**异步执行**的：
1. 调用此接口后，流水线状态变为 `running`，并被提交到后台执行队列
2. 实际执行由后台 worker 协程处理，包括：
   - 查找可用的 Worker 节点
   - 分配 GPU/CPU/内存资源
   - 依次执行 6 个阶段（每个阶段可能需要数分钟到数小时）
   - 调度 EasyDistill 容器完成推理、训练、评估等任务
3. 前端需要通过**轮询**以下接口来获取实时状态：
   - `GET /api/v1/pipelines/{id}` - 获取流水线整体状态和当前阶段
   - `GET /api/v1/pipelines/{id}/stages` - 获取各阶段的详细状态
   - `GET /api/v1/pipelines/{id}/stages/{stage_id}/logs/stream` - 获取阶段实时日志

**前端实现建议**：
```javascript
// 启动流水线
async function startPipeline(pipelineId) {
  await axios.post(`/api/v1/pipelines/${pipelineId}/start`);

  // 启动轮询，每 3 秒获取一次状态
  const timer = setInterval(async () => {
    const { data } = await axios.get(`/api/v1/pipelines/${pipelineId}`);

    // 更新 UI 显示当前状态
    updatePipelineStatus(data.data);

    // 如果流水线完成或失败，停止轮询
    if (['succeeded', 'failed', 'canceled'].includes(data.data.status)) {
      clearInterval(timer);
    }
  }, 3000);
}
```

#### 3.3 取消流水线

**请求**

```http
POST /api/v1/pipelines/{pipeline_id}/cancel
```

**响应**

```json
{
  "code": 200,
  "message": "流水线已取消"
}
```

#### 3.4 获取流水线详情

**请求**

```http
GET /api/v1/pipelines/{pipeline_id}
```

**响应**

```json
{
  "code": 200,
  "message": "获取流水线成功",
  "data": {
    "id": "pipeline-uuid",
    "project_id": "project-uuid",
    "dataset_id": "dataset-uuid",
    "status": "running",
    "current_stage": 3,
    "started_at": "2024-01-01T00:00:00Z",
    "error_message": null
  }
}
```

#### 3.5 获取流水线列表

**请求**

```http
GET /api/v1/pipelines?project_id={project_id}&status=running&page=1&page_size=10
```

**响应**

```json
{
  "code": 200,
  "message": "获取流水线列表成功",
  "data": {
    "items": [
      {
        "id": "pipeline-uuid",
        "project_id": "project-uuid",
        "status": "running",
        "current_stage": 3,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "page_size": 10
  }
}
```

#### 3.6 获取阶段列表

**请求**

```http
GET /api/v1/pipelines/{pipeline_id}/stages
```

**响应**

```json
{
  "code": 200,
  "message": "获取阶段列表成功",
  "data": [
    {
      "id": "stage-uuid-1",
      "pipeline_run_id": "pipeline-uuid",
      "stage_type": "teacher_config",
      "stage_order": 1,
      "status": "succeeded",
      "node_name": "worker-1",
      "started_at": "2024-01-01T00:00:00Z",
      "finished_at": "2024-01-01T00:01:00Z"
    },
    {
      "id": "stage-uuid-2",
      "pipeline_run_id": "pipeline-uuid",
      "stage_type": "dataset_build",
      "stage_order": 2,
      "status": "succeeded",
      "started_at": "2024-01-01T00:01:00Z",
      "finished_at": "2024-01-01T00:02:00Z"
    },
    {
      "id": "stage-uuid-3",
      "pipeline_run_id": "pipeline-uuid",
      "stage_type": "teacher_infer",
      "stage_order": 3,
      "status": "running",
      "node_name": "worker-1",
      "started_at": "2024-01-01T00:02:00Z",
      "finished_at": null
    },
    {
      "id": "stage-uuid-4",
      "stage_type": "data_govern",
      "stage_order": 4,
      "status": "pending"
    },
    {
      "id": "stage-uuid-5",
      "stage_type": "student_train",
      "stage_order": 5,
      "status": "pending"
    },
    {
      "id": "stage-uuid-6",
      "stage_type": "evaluate",
      "stage_order": 6,
      "status": "pending"
    }
  ]
}
```

---

### 4. 日志管理 API

#### 4.1 获取阶段完整日志

**请求**

```http
GET /api/v1/pipelines/{pipeline_id}/stages/{stage_id}/logs
```

获取指定阶段的完整日志内容。

**响应**

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

**前端使用示例**

```javascript
async function getFullLogs(pipelineId, stageId) {
  try {
    const response = await axios.get(
      `/api/v1/pipelines/${pipelineId}/stages/${stageId}/logs`
    );
    if (response.data.code === 200) {
      return response.data.data.logs;
    }
  } catch (error) {
    console.error('获取日志失败:', error);
  }
}
```

#### 4.2 获取阶段实时日志

**请求**

```http
GET /api/v1/pipelines/{pipeline_id}/stages/{stage_id}/logs/stream?tail=100
```

获取指定阶段的实时日志（最后N行），适合轮询显示实时日志。

**查询参数**:
- `tail`: 返回最后N行日志（默认 100）

**响应**

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

**前端使用示例（轮询实时日志）**

```javascript
// 在流水线详情页面组件中实现实时日志
export default {
  data() {
    return {
      logs: '',
      logTimer: null,
      currentStage: null
    };
  },
  methods: {
    async fetchRealtimeLogs() {
      if (!this.currentStage || this.currentStage.status !== 'running') {
        return;
      }

      try {
        const response = await axios.get(
          `/api/v1/pipelines/${this.pipelineId}/stages/${this.currentStage.id}/logs/stream?tail=100`
        );

        if (response.data.code === 200) {
          this.logs = response.data.data.logs;
        }
      } catch (error) {
        console.error('获取实时日志失败:', error);
      }
    },

    startLogPolling() {
      // 每3秒获取一次实时日志
      this.logTimer = setInterval(() => {
        this.fetchRealtimeLogs();
      }, 3000);
    },

    stopLogPolling() {
      if (this.logTimer) {
        clearInterval(this.logTimer);
        this.logTimer = null;
      }
    }
  },

  mounted() {
    this.startLogPolling();
  },

  beforeDestroy() {
    this.stopLogPolling();
  }
};
```

**Vue 模板示例**

```vue
<template>
  <el-card title="实时日志">
    <div class="log-container">
      <pre class="log-content">{{ logs || '暂无日志...' }}</pre>
    </div>
    <div class="log-actions">
      <el-button @click="fetchRealtimeLogs" size="small">刷新</el-button>
      <el-button @click="viewFullLogs" size="small">查看完整日志</el-button>
      <el-button @click="downloadLogs" size="small">下载日志</el-button>
    </div>
  </el-card>
</template>

<style scoped>
.log-container {
  max-height: 500px;
  overflow-y: auto;
  background-color: #1e1e1e;
  padding: 16px;
  border-radius: 4px;
}

.log-content {
  color: #d4d4d4;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.log-actions {
  margin-top: 16px;
  text-align: right;
}
</style>
```

#### 4.3 下载阶段日志文件

**请求**

```http
GET /api/v1/pipelines/{pipeline_id}/stages/{stage_id}/logs/download
```

下载指定阶段的日志文件。直接返回日志文件，浏览器会提示下载。

**前端使用示例**

```javascript
function downloadLogs(pipelineId, stageId) {
  // 方法1: 直接使用 window.location
  window.location.href =
    `/api/v1/pipelines/${pipelineId}/stages/${stageId}/logs/download`;

  // 方法2: 使用 a 标签下载
  const link = document.createElement('a');
  link.href = `/api/v1/pipelines/${pipelineId}/stages/${stageId}/logs/download`;
  link.download = `stage_${stageId}_log.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}
```

**使用注意事项**

1. **实时日志轮询频率**: 建议每 3-5 秒轮询一次，避免频繁请求造成服务器压力
2. **停止轮询**: 当阶段状态变为 `succeeded`、`failed` 或 `canceled` 时，应停止日志轮询
3. **日志自动滚动**: 建议在日志容器中实现自动滚动到底部的功能
4. **大文件处理**: 完整日志可能很大，建议先使用实时日志接口，需要时再获取完整日志

---

### 5. 资源管理 API

#### 5.1 获取 Worker 节点列表

**请求**

```http
GET /api/v1/resources/nodes
```

**响应**

```json
{
  "code": 200,
  "message": "获取节点列表成功",
  "data": [
    {
      "node_name": "worker-1",
      "node_addr": "192.168.1.10:50052",
      "status": "online",
      "total_gpu": 4,
      "available_gpu": 2,
      "total_memory_gb": 128,
      "total_cpu": 32,
      "last_heartbeat": "2024-01-01T00:00:00Z"
    },
    {
      "node_name": "worker-2",
      "node_addr": "192.168.1.11:50052",
      "status": "busy",
      "total_gpu": 8,
      "available_gpu": 0,
      "total_memory_gb": 256,
      "total_cpu": 64,
      "last_heartbeat": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### 5.2 获取节点详情

**请求**

```http
GET /api/v1/resources/nodes/{node_name}
```

**响应**

```json
{
  "code": 200,
  "message": "获取节点信息成功",
  "data": {
    "node_name": "worker-1",
    "node_addr": "192.168.1.10:50052",
    "status": "online",
    "total_gpu": 4,
    "available_gpu": 2,
    "total_memory_gb": 128,
    "total_cpu": 32,
    "last_heartbeat": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

---

### 6. 健康检查 API

**请求**

```http
GET /health
```

**响应**

```json
{
  "status": "ok"
}
```

---

## 数据结构定义

### 枚举类型

#### 流水线状态 (PipelineStatus)

```typescript
type PipelineStatus =
  | 'pending'    // 等待中
  | 'scheduled'  // 已调度
  | 'preparing'  // 准备中
  | 'running'    // 运行中
  | 'succeeded'  // 成功
  | 'failed'     // 失败
  | 'canceled';  // 已取消
```

#### 阶段类型 (StageType)

```typescript
type StageType =
  | 'teacher_config'  // 教师模型配置
  | 'dataset_build'   // 蒸馏数据构建
  | 'teacher_infer'   // 教师推理与样本生成
  | 'data_govern'     // 蒸馏数据治理
  | 'student_train'   // 学生模型训练
  | 'evaluate';       // 蒸馏效果评估
```

#### 模型提供者类型 (ProviderType)

```typescript
type ProviderType =
  | 'api'    // API 型教师模型（如 OpenAI）
  | 'local'; // 本地教师模型
```

### TypeScript 接口定义

#### Project

```typescript
interface Project {
  id: string;
  name: string;
  description: string;
  business_scenario: string;
  teacher_model_config: ModelConfig;
  student_model_config: ModelConfig;
  evaluation_config?: EvaluationConfig;
  created_at: string; // ISO 8601
  updated_at: string; // ISO 8601
}
```

#### ModelConfig

```typescript
interface ModelConfig {
  provider_type: ProviderType;
  model_name: string;
  endpoint?: string;         // API 端点（API 型）
  api_secret_ref?: string;   // API 密钥引用
  temperature?: number;
  max_tokens?: number;
  concurrency?: number;      // 并发数
  timeout_seconds?: number;
  extra_params?: Record<string, any>;
}
```

#### EvaluationConfig

```typescript
interface EvaluationConfig {
  metrics: string[];         // ["bleu", "rouge", "accuracy"]
  test_set_ratio: number;    // 测试集比例 0.0-1.0
  extra_params?: Record<string, any>;
}
```

#### Dataset

```typescript
interface Dataset {
  id: string;
  project_id: string;
  name: string;
  description: string;
  source_type: 'upload' | 'import' | 'generated';
  file_path: string;
  record_count: number;
  created_at: string; // ISO 8601
}
```

#### PipelineRun

```typescript
interface PipelineRun {
  id: string;
  project_id: string;
  dataset_id: string;
  status: PipelineStatus;
  current_stage: number;      // 当前阶段序号 (0-6)
  trigger_mode: 'manual' | 'scheduled';
  training_config: TrainingConfig;
  resource_request: ResourceRequest;
  error_message?: string;
  created_at: string;         // ISO 8601
  started_at?: string;        // ISO 8601
  finished_at?: string;       // ISO 8601
  updated_at: string;         // ISO 8601
}
```

#### TrainingConfig

```typescript
interface TrainingConfig {
  num_train_epochs: number;
  per_device_train_batch_size: number;
  gradient_accumulation_steps?: number;
  learning_rate: number;
  weight_decay?: number;
  warmup_ratio?: number;
  lr_scheduler_type?: 'cosine' | 'linear';
  save_steps?: number;
  logging_steps?: number;
  max_length?: number;
  lora_config?: LoRAConfig;
}
```

#### LoRAConfig

```typescript
interface LoRAConfig {
  enabled: boolean;
  r?: number;                // LoRA rank
  alpha?: number;            // LoRA alpha
  dropout?: number;
  target_modules?: string[]; // ["q_proj", "v_proj"]
}
```

#### ResourceRequest

```typescript
interface ResourceRequest {
  gpu_count: number;
  gpu_device_ids?: string;   // "0,1,2" - 指定使用的 GPU 设备 ID
  gpu_type?: string;         // "A100", "V100"
  memory_gb?: number;
  cpu_cores?: number;
}
```

**gpu_device_ids 说明**:
- 如果指定，格式为逗号分隔的设备 ID，如 `"0,1,2"`
- 优先级高于 `gpu_count`，即如果同时指定，使用 `gpu_device_ids`
- 未指定则由系统根据 `gpu_count` 自动分配

#### StageRun

```typescript
interface StageRun {
  id: string;
  pipeline_run_id: string;
  stage_type: StageType;
  stage_order: number;       // 阶段序号 (1-6)
  status: PipelineStatus;
  container_id?: string;
  node_name?: string;
  config_path?: string;
  input_manifest?: Record<string, string>;
  output_manifest?: Record<string, string>;
  metrics?: Record<string, any>;
  log_path?: string;
  retry_count: number;
  error_message?: string;
  started_at?: string;       // ISO 8601
  finished_at?: string;      // ISO 8601
  created_at: string;        // ISO 8601
  updated_at: string;        // ISO 8601
}
```

#### WorkerNode

```typescript
interface WorkerNode {
  node_name: string;
  node_addr: string;
  status: 'online' | 'offline' | 'busy';
  total_gpu: number;
  available_gpu: number;
  total_memory_gb: number;
  total_cpu: number;
  last_heartbeat: string;    // ISO 8601
  updated_at: string;        // ISO 8601
}
```

---

## 页面布局建议

### 1. 整体布局

建议采用**左侧导航栏 + 顶部面包屑 + 主内容区**的经典布局：

```
┌────────────────────────────────────────────────────────────────┐
│  Logo   │  项目名称                          用户头像  通知  设置  │
├─────────┼───────────────────────────────────────────────────────┤
│         │  首页 > 项目管理 > 项目详情                              │
│  导航栏  ├───────────────────────────────────────────────────────┤
│         │                                                        │
│  项目    │                                                        │
│  数据集  │              主内容区                                   │
│  流水线  │                                                        │
│  资源    │                                                        │
│  设置    │                                                        │
│         │                                                        │
└─────────┴───────────────────────────────────────────────────────┘
```

### 2. 项目管理页面

#### 2.1 项目列表页

**布局要点**：
- 顶部：搜索框 + 筛选器 + "创建项目"按钮
- 主体：卡片式或表格式列表
- 每个项目卡片显示：
  - 项目名称（点击进入详情）
  - 简要描述
  - 教师模型 → 学生模型
  - 创建时间
  - 快捷操作：查看、编辑、删除

**推荐组件库实现（Element UI）**：

```vue
<template>
  <div>
    <!-- 顶部操作栏 -->
    <div style="margin-bottom: 16px">
      <el-input
        placeholder="搜索项目名称"
        v-model="searchText"
        prefix-icon="el-icon-search"
        style="width: 300px; margin-right: 16px"
      />
      <el-button type="primary" icon="el-icon-plus" @click="createProject">
        创建项目
      </el-button>
    </div>

    <!-- 项目卡片列表 -->
    <el-row :gutter="16">
      <el-col :span="8" v-for="project in projects" :key="project.id">
        <el-card :body-style="{ padding: '20px' }">
          <div slot="header" class="clearfix">
            <span>{{ project.name }}</span>
            <el-button style="float: right; padding: 3px 0" type="text" @click="viewProject(project)">
              查看
            </el-button>
          </div>
          <p>{{ project.description }}</p>
          <p>
            教师: {{ project.teacher_model_config.model_name }} →
            学生: {{ project.student_model_config.model_name }}
          </p>
          <p>创建时间: {{ formatDate(project.created_at) }}</p>
          <div style="text-align: right">
            <el-button type="text" @click="editProject(project)">编辑</el-button>
            <el-button type="text" class="danger" @click="deleteProject(project)">删除</el-button>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script>
export default {
  data() {
    return {
      searchText: '',
      projects: []
    };
  },
  methods: {
    formatDate(date) {
      // 格式化日期的方法
      return new Date(date).toLocaleString('zh-CN');
    },
    createProject() {
      // 创建项目逻辑
    },
    viewProject(project) {
      // 查看项目逻辑
    },
    editProject(project) {
      // 编辑项目逻辑
    },
    deleteProject(project) {
      // 删除项目逻辑
    }
  }
};
</script>
```

#### 2.2 创建/编辑项目页

**布局要点**：
- 表单式布局，分为几个区域：
  1. 基本信息（项目名称、描述、业务场景）
  2. 教师模型配置（提供者类型、模型名称、API 配置）
  3. 学生模型配置（模型名称、模型路径）
  4. 评估配置（评估指标、测试集比例）

**表单示例**：

```vue
<template>
  <el-form :model="form" label-position="top">
    <!-- 基本信息 -->
    <el-form-item label="项目名称" prop="name" required>
      <el-input v-model="form.name" placeholder="例如：客服问答蒸馏" />
    </el-form-item>

    <el-form-item label="项目描述" prop="description">
      <el-input
        type="textarea"
        :rows="3"
        v-model="form.description"
      />
    </el-form-item>

    <!-- 教师模型配置 -->
    <el-divider>教师模型配置</el-divider>

    <el-form-item label="提供者类型" prop="teacher_model_config.provider_type">
      <el-select v-model="form.teacher_model_config.provider_type">
        <el-option value="api" label="API 型（OpenAI/Claude）" />
        <el-option value="local" label="本地模型" />
      </el-select>
    </el-form-item>

    <el-form-item label="模型名称" prop="teacher_model_config.model_name">
      <el-input
        v-model="form.teacher_model_config.model_name"
        placeholder="例如：Qwen/Qwen2.5-7B-Instruct"
      />
    </el-form-item>

    <el-form-item label="API 端点" prop="teacher_model_config.endpoint">
      <el-input
        v-model="form.teacher_model_config.endpoint"
        placeholder="https://api.openai.com/v1/chat/completions"
      />
    </el-form-item>

    <el-form-item label="Temperature" prop="teacher_model_config.temperature">
      <el-input-number
        v-model="form.teacher_model_config.temperature"
        :min="0"
        :max="2"
        :step="0.1"
      />
    </el-form-item>

    <!-- 学生模型配置 -->
    <el-divider>学生模型配置</el-divider>

    <el-form-item label="模型名称" prop="student_model_config.model_name">
      <el-input
        v-model="form.student_model_config.model_name"
        placeholder="例如：Qwen/Qwen2.5-0.5B-Instruct"
      />
    </el-form-item>

    <!-- 评估配置 -->
    <el-divider>评估配置</el-divider>

    <el-form-item label="评估指标" prop="evaluation_config.metrics">
      <el-select v-model="form.evaluation_config.metrics" multiple>
        <el-option value="bleu" label="BLEU" />
        <el-option value="rouge" label="ROUGE" />
        <el-option value="accuracy" label="准确率" />
      </el-select>
    </el-form-item>
  </el-form>
</template>

<script>
export default {
  data() {
    return {
      form: {
        name: '',
        description: '',
        teacher_model_config: {
          provider_type: '',
          model_name: '',
          endpoint: '',
          temperature: 0.7
        },
        student_model_config: {
          model_name: ''
        },
        evaluation_config: {
          metrics: []
        }
      }
    };
  }
};
</script>
```

#### 2.3 项目详情页

**布局要点**：
- 顶部：项目基本信息卡片
- Tab 切换：
  - **概览** - 项目配置摘要、统计信息
  - **数据集** - 该项目下的所有数据集
  - **流水线** - 该项目下的所有流水线运行记录
  - **设置** - 编辑项目配置

**推荐布局**：

```vue
<template>
  <div>
    <!-- 项目头部信息 -->
    <el-card>
      <el-row :gutter="16">
        <el-col :span="18">
          <h2>{{ project.name }}</h2>
          <p>{{ project.description }}</p>
        </el-col>
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">流水线总数</div>
            <div class="statistic-value">10</div>
          </div>
          <div class="statistic">
            <div class="statistic-title">成功次数</div>
            <div class="statistic-value">8</div>
          </div>
        </el-col>
      </el-row>
    </el-card>

    <!-- Tab 导航 -->
    <el-tabs v-model="activeTab">
      <el-tab-pane label="概览" name="overview">
        <el-descriptions>
          <el-descriptions-item label="教师模型">
            {{ project.teacher_model_config.model_name }}
          </el-descriptions-item>
          <el-descriptions-item label="学生模型">
            {{ project.student_model_config.model_name }}
          </el-descriptions-item>
          <el-descriptions-item label="业务场景">
            {{ project.business_scenario }}
          </el-descriptions-item>
        </el-descriptions>
      </el-tab-pane>

      <el-tab-pane label="数据集" name="datasets">
        <!-- 数据集列表 -->
      </el-tab-pane>

      <el-tab-pane label="流水线" name="pipelines">
        <!-- 流水线列表 -->
      </el-tab-pane>

      <el-tab-pane label="设置" name="settings">
        <!-- 项目编辑表单 -->
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script>
export default {
  data() {
    return {
      activeTab: 'overview',
      project: {}
    };
  }
};
</script>
```

---

### 3. 数据集管理页面

#### 3.1 数据集列表

**布局要点**：
- 表格展示：数据集名称、来源类型、记录数、创建时间
- 操作列：查看、下载、删除
- 顶部按钮：上传数据集

#### 3.2 上传数据集

**布局要点**：
- 文件上传组件（拖拽上传）
- 数据预览（上传后显示前 10 条数据）
- 数据格式说明

**推荐实现**：

```vue
<template>
  <div>
    <el-upload
      class="upload-demo"
      drag
      action="/api/v1/datasets"
      accept=".json,.jsonl,.csv"
      :before-upload="handleUpload"
    >
      <i class="el-icon-upload"></i>
      <div class="el-upload__text">将文件拖到此处，或<em>点击上传</em></div>
      <div class="el-upload__tip" slot="tip">
        支持 .json, .jsonl, .csv 格式
      </div>
    </el-upload>

    <!-- 数据预览 -->
    <el-card v-if="previewData && previewData.length" title="数据预览（前 10 条）" style="margin-top: 16px">
      <el-table
        :data="previewData"
        border
        style="width: 100%"
      >
        <el-table-column
          v-for="col in previewColumns"
          :key="col.prop"
          :prop="col.prop"
          :label="col.label"
        />
      </el-table>
    </el-card>
  </div>
</template>

<script>
export default {
  data() {
    return {
      previewData: null,
      previewColumns: []
    };
  },
  methods: {
    handleUpload(file) {
      // 处理文件上传逻辑
      return true;
    }
  }
};
</script>
```

---

### 4. 流水线管理页面

#### 4.1 流水线列表

**布局要点**：
- 表格展示：流水线 ID、项目名称、状态、当前阶段、开始时间
- 状态标签颜色：
  - `pending` - 灰色
  - `running` - 蓝色（闪烁动画）
  - `succeeded` - 绿色
  - `failed` - 红色
  - `canceled` - 橙色

**状态组件示例**：

```vue
<template>
  <el-tag :type="statusConfig[status].type">
    {{ statusConfig[status].text }}
  </el-tag>
</template>

<script>
export default {
  props: {
    status: {
      type: String,
      required: true
    }
  },
  data() {
    return {
      statusConfig: {
        pending: { type: 'info', text: '等待中' },
        scheduled: { type: 'primary', text: '已调度' },
        preparing: { type: 'primary', text: '准备中' },
        running: { type: 'primary', text: '运行中' },
        succeeded: { type: 'success', text: '成功' },
        failed: { type: 'danger', text: '失败' },
        canceled: { type: 'warning', text: '已取消' }
      }
    };
  }
};
</script>
```

#### 4.2 创建流水线页

**布局要点**：
- Step 1: 选择项目和数据集
- Step 2: 配置训练参数
- Step 3: 配置资源需求
- Step 4: 确认并创建

**Steps 组件示例**：

```vue
<template>
  <div>
    <el-steps :active="currentStep" finish-status="success">
      <el-step
        v-for="(step, index) in steps"
        :key="index"
        :title="step.title"
      />
    </el-steps>

    <div style="margin-top: 24px">
      <!-- Step 1: 选择项目和数据集 -->
      <el-form v-if="currentStep === 0" :model="pipelineForm" label-position="top">
        <el-form-item label="项目" prop="project_id">
          <el-select v-model="pipelineForm.project_id" placeholder="选择项目">
            <el-option
              v-for="p in projects"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="数据集" prop="dataset_id">
          <el-select v-model="pipelineForm.dataset_id" placeholder="选择数据集">
            <el-option
              v-for="d in datasets"
              :key="d.id"
              :label="d.name"
              :value="d.id"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <!-- Step 2: 训练参数 -->
      <el-form v-if="currentStep === 1" :model="pipelineForm" label-position="top">
        <el-form-item label="训练轮数" prop="training_config.num_train_epochs">
          <el-input-number
            v-model="pipelineForm.training_config.num_train_epochs"
            :min="1"
            :max="100"
          />
        </el-form-item>
        <el-form-item label="学习率" prop="training_config.learning_rate">
          <el-input-number
            v-model="pipelineForm.training_config.learning_rate"
            :min="0"
            :max="1"
            :step="0.00001"
          />
        </el-form-item>
      </el-form>

      <!-- Step 3: 资源需求 -->
      <el-form v-if="currentStep === 2" :model="pipelineForm" label-position="top">
        <el-form-item label="GPU 数量" prop="resource_request.gpu_count">
          <el-input-number
            v-model="pipelineForm.resource_request.gpu_count"
            :min="0"
            :max="8"
          />
        </el-form-item>
        <el-form-item label="GPU 设备 ID" prop="resource_request.gpu_device_ids">
          <el-input
            v-model="pipelineForm.resource_request.gpu_device_ids"
            placeholder="0,1,2"
          >
            <template #suffix>
              <el-tooltip content="指定使用的 GPU 设备，如 '0,1' 表示使用 GPU 0 和 1，留空则自动分配">
                <i class="el-icon-question"></i>
              </el-tooltip>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item label="内存 (GB)" prop="resource_request.memory_gb">
          <el-input-number
            v-model="pipelineForm.resource_request.memory_gb"
            :min="8"
            :max="512"
          />
        </el-form-item>
      </el-form>
    </div>

    <div style="margin-top: 24px">
      <el-button v-if="currentStep > 0" @click="currentStep--">上一步</el-button>
      <el-button v-if="currentStep < steps.length - 1" type="primary" @click="currentStep++">
        下一步
      </el-button>
      <el-button v-if="currentStep === steps.length - 1" type="primary" @click="handleSubmit">
        创建流水线
      </el-button>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      currentStep: 0,
      projects: [],
      datasets: [],
      steps: [
        { title: '选择项目和数据集' },
        { title: '训练参数' },
        { title: '资源需求' }
      ],
      pipelineForm: {
        project_id: '',
        dataset_id: '',
        training_config: {
          num_train_epochs: 3,
          learning_rate: 0.0001
        },
        resource_request: {
          gpu_count: 1,
          gpu_device_ids: '',
          memory_gb: 32,
          cpu_cores: 8
        }
      }
    };
  },
  methods: {
    handleSubmit() {
      // 提交创建流水线
    }
  }
};
</script>
```

#### 4.3 流水线详情页 ⭐ 重点

这是最核心的页面，需要重点设计！

**布局要点**：
- **顶部卡片**：流水线基本信息（ID、状态、项目、数据集、开始/结束时间）
- **6 阶段进度条**：横向 Step 组件，显示每个阶段的状态
- **当前阶段详情**：
  - 阶段名称、状态、执行节点
  - 实时日志（自动滚动）
  - 阶段指标（如果有）
- **操作按钮**：取消流水线、查看完整日志、下载报告

**推荐布局**：

```vue
<template>
  <div style="padding: 24px">
    <!-- 流水线头部 -->
    <el-card>
      <el-row :gutter="16">
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">流水线状态</div>
            <div class="statistic-value" :style="{ color: pipeline.status === 'succeeded' ? '#67C23A' : '#F56C6C' }">
              {{ pipeline.status }}
            </div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">当前阶段</div>
            <div class="statistic-value">{{ pipeline.current_stage }}/6</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">项目名称</div>
            <div class="statistic-value">{{ pipeline.project_id }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <el-button v-if="pipeline.status === 'running'" type="danger" @click="handleCancel">
            取消流水线
          </el-button>
        </el-col>
      </el-row>
    </el-card>

    <!-- 6 阶段进度 -->
    <el-card title="流水线进度" style="margin-top: 16px">
      <el-steps :active="pipeline.current_stage - 1" finish-status="success">
        <el-step
          v-for="stage in stages"
          :key="stage.id"
          :title="getStageTitle(stage.stage_type)"
          :status="getStepStatus(stage.status)"
        >
          <template #description>
            <div>
              <el-tag :type="getStatusColor(stage.status)">
                {{ getStatusText(stage.status) }}
              </el-tag>
            </div>
          </template>
        </el-step>
      </el-steps>
    </el-card>

    <!-- 当前阶段详情 -->
    <el-card v-if="currentStage" :title="`当前阶段: ${getStageTitle(currentStage.stage_type)}`" style="margin-top: 16px">
      <el-descriptions :column="2">
        <el-descriptions-item label="阶段状态">
          <el-tag :type="getStatusColor(currentStage.status)">
            {{ getStatusText(currentStage.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="执行节点">
          {{ currentStage.node_name || '未分配' }}
        </el-descriptions-item>
        <el-descriptions-item label="开始时间">
          {{ currentStage.started_at || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="容器 ID">
          {{ currentStage.container_id || '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 执行历史 -->
    <el-card title="执行历史" style="margin-top: 16px">
      <el-timeline>
        <el-timeline-item
          v-for="stage in stages.filter(s => s.started_at)"
          :key="stage.id"
          :color="stage.status === 'succeeded' ? 'green' : stage.status === 'failed' ? 'red' : 'blue'"
        >
          <p><strong>{{ getStageTitle(stage.stage_type) }}</strong></p>
          <p>
            状态: <el-tag :type="getStatusColor(stage.status)">{{ getStatusText(stage.status) }}</el-tag>
          </p>
          <p>开始: {{ stage.started_at }}</p>
          <p v-if="stage.finished_at">结束: {{ stage.finished_at }}</p>
          <el-alert
            v-if="stage.error_message"
            type="error"
            :title="stage.error_message"
            :closable="false"
            show-icon
          />
        </el-timeline-item>
      </el-timeline>
    </el-card>
  </div>
</template>

<script>
import axios from 'axios';

export default {
  data() {
    return {
      pipeline: null,
      stages: [],
      timer: null
    };
  },
  computed: {
    currentStage() {
      return this.stages.find(s => s.status === 'running')
        || this.stages.find(s => s.stage_order === this.pipeline?.current_stage);
    }
  },
  created() {
    this.fetchPipelineDetail();
    this.fetchStages();
    // 轮询更新
    this.timer = setInterval(() => {
      if (this.pipeline?.status === 'running') {
        this.fetchPipelineDetail();
        this.fetchStages();
      }
    }, 3000);
  },
  beforeDestroy() {
    if (this.timer) {
      clearInterval(this.timer);
    }
  },
  methods: {
    async fetchPipelineDetail() {
      try {
        const response = await axios.get(`/api/v1/pipelines/${this.$route.params.id}`);
        this.pipeline = response.data.data;
      } catch (error) {
        this.$message.error('获取流水线详情失败');
      }
    },
    async fetchStages() {
      try {
        const response = await axios.get(`/api/v1/pipelines/${this.$route.params.id}/stages`);
        this.stages = response.data.data;
      } catch (error) {
        this.$message.error('获取阶段列表失败');
      }
    },
    async handleCancel() {
      try {
        await axios.post(`/api/v1/pipelines/${this.$route.params.id}/cancel`);
        this.$message.success('流水线已取消');
        this.fetchPipelineDetail();
      } catch (error) {
        this.$message.error('取消失败');
      }
    },
    getStageTitle(stageType) {
      const titles = {
        'teacher_config': '教师模型配置',
        'dataset_build': '蒸馏数据构建',
        'teacher_infer': '教师推理',
        'data_govern': '数据治理',
        'student_train': '学生训练',
        'evaluate': '效果评估'
      };
      return titles[stageType] || stageType;
    },
    getStatusColor(status) {
      const colors = {
        'pending': 'info',
        'running': 'primary',
        'succeeded': 'success',
        'failed': 'danger',
        'canceled': 'warning'
      };
      return colors[status] || 'info';
    },
    getStatusText(status) {
      const texts = {
        'pending': '等待中',
        'scheduled': '已调度',
        'preparing': '准备中',
        'running': '运行中',
        'succeeded': '成功',
        'failed': '失败',
        'canceled': '已取消'
      };
      return texts[status] || status;
    },
    getStepStatus(status) {
      if (status === 'succeeded') return 'success';
      if (status === 'failed') return 'error';
      if (status === 'running') return 'process';
      return 'wait';
    }
  }
};
</script>

<style scoped>
.statistic {
  margin-bottom: 16px;
}
.statistic-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}
.statistic-value {
  font-size: 24px;
  font-weight: bold;
}
</style>
```

---

### 5. 资源管理页面

#### 5.1 Worker 节点列表

**布局要点**：
- 卡片式展示每个 Worker 节点
- 每个卡片显示：
  - 节点名称、地址、状态（在线/离线/忙碌）
  - GPU 使用情况（进度条）
  - 内存使用情况（进度条）
  - 最后心跳时间

**推荐布局**：

```vue
<template>
  <el-row :gutter="16">
    <el-col :span="8" v-for="node in nodes" :key="node.node_name">
      <WorkerNodeCard :node="node" />
    </el-col>
  </el-row>
</template>

<script>
import WorkerNodeCard from './WorkerNodeCard.vue';

export default {
  components: {
    WorkerNodeCard
  },
  data() {
    return {
      nodes: []
    };
  }
};
</script>
```

**WorkerNodeCard 组件**：

```vue
<template>
  <el-card>
    <div slot="header" class="clearfix">
      <span>
        <el-badge :status="node.status === 'online' ? 'success' : 'danger'" />
        {{ node.node_name }}
      </span>
      <el-tag style="float: right">{{ node.status }}</el-tag>
    </div>
    <p>地址: {{ node.node_addr }}</p>

    <el-divider />

    <div>
      <p>GPU 使用情况</p>
      <el-progress
        :percentage="gpuUsage"
        :status="gpuUsage > 80 ? 'exception' : 'success'"
        :format="() => `${node.total_gpu - node.available_gpu}/${node.total_gpu}`"
      />
    </div>

    <div style="margin-top: 16px">
      <p>资源配置</p>
      <el-row>
        <el-col :span="12">总内存: {{ node.total_memory_gb }} GB</el-col>
        <el-col :span="12">CPU 核心: {{ node.total_cpu }}</el-col>
      </el-row>
    </div>

    <div style="margin-top: 16px">
      <p>最后心跳: {{ formatRelativeTime(node.last_heartbeat) }}</p>
    </div>
  </el-card>
</template>

<script>
export default {
  props: {
    node: {
      type: Object,
      required: true
    }
  },
  computed: {
    gpuUsage() {
      return ((this.node.total_gpu - this.node.available_gpu) / this.node.total_gpu) * 100;
    }
  },
  methods: {
    formatRelativeTime(timestamp) {
      // 格式化相对时间
      const diff = Date.now() - new Date(timestamp).getTime();
      const seconds = Math.floor(diff / 1000);
      if (seconds < 60) return `${seconds}秒前`;
      const minutes = Math.floor(seconds / 60);
      if (minutes < 60) return `${minutes}分钟前`;
      const hours = Math.floor(minutes / 60);
      return `${hours}小时前`;
    }
  }
};
</script>
```

---

## 状态管理建议

### 使用 Vuex

**推荐使用 Vuex** 进行全局状态管理：

```javascript
// store/modules/project.js
import axios from 'axios';

const state = {
  projects: [],
  currentProject: null,
  loading: false
};

const mutations = {
  SET_PROJECTS(state, projects) {
    state.projects = projects;
  },
  SET_CURRENT_PROJECT(state, project) {
    state.currentProject = project;
  },
  SET_LOADING(state, loading) {
    state.loading = loading;
  }
};

const actions = {
  async fetchProjects({ commit }) {
    commit('SET_LOADING', true);
    try {
      const response = await axios.get('/api/v1/projects');
      commit('SET_PROJECTS', response.data.data.items);
    } catch (error) {
      this._vm.$message.error('获取项目列表失败');
    } finally {
      commit('SET_LOADING', false);
    }
  },

  async fetchProject({ commit }, id) {
    try {
      const response = await axios.get(`/api/v1/projects/${id}`);
      commit('SET_CURRENT_PROJECT', response.data.data);
    } catch (error) {
      this._vm.$message.error('获取项目失败');
    }
  },

  async createProject({ dispatch }, project) {
    try {
      const response = await axios.post('/api/v1/projects', project);
      if (response.data.code === 200) {
        this._vm.$message.success('项目创建成功');
        dispatch('fetchProjects');
      }
    } catch (error) {
      this._vm.$message.error('创建项目失败');
    }
  },

  async updateProject({ dispatch }, { id, updates }) {
    try {
      await axios.put(`/api/v1/projects/${id}`, updates);
      this._vm.$message.success('项目更新成功');
      dispatch('fetchProjects');
    } catch (error) {
      this._vm.$message.error('更新项目失败');
    }
  },

  async deleteProject({ dispatch }, id) {
    try {
      await axios.delete(`/api/v1/projects/${id}`);
      this._vm.$message.success('项目删除成功');
      dispatch('fetchProjects');
    } catch (error) {
      this._vm.$message.error('删除项目失败');
    }
  }
};

const getters = {
  projects: state => state.projects,
  currentProject: state => state.currentProject,
  loading: state => state.loading
};

export default {
  namespaced: true,
  state,
  mutations,
  actions,
  getters
};
```

**在组件中使用**：

```vue
<template>
  <div>
    <el-table
      v-loading="loading"
      :data="projects"
      border
      style="width: 100%"
    >
      <!-- 表格列定义 -->
    </el-table>
  </div>
</template>

<script>
import { mapGetters, mapActions } from 'vuex';

export default {
  computed: {
    ...mapGetters('project', ['projects', 'loading'])
  },
  methods: {
    ...mapActions('project', ['fetchProjects'])
  },
  created() {
    this.fetchProjects();
  }
};
</script>
```

---

## 开发示例

### 完整的流水线详情页实现

```vue
<template>
  <div v-loading="loading" style="padding: 24px">
    <!-- 流水线头部 -->
    <el-card v-if="pipeline">
      <el-row :gutter="16">
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">流水线状态</div>
            <div class="statistic-value" :style="{ color: pipeline.status === 'succeeded' ? '#67C23A' : '#F56C6C' }">
              {{ pipeline.status }}
            </div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">当前阶段</div>
            <div class="statistic-value">{{ pipeline.current_stage }}/6</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="statistic">
            <div class="statistic-title">项目名称</div>
            <div class="statistic-value">{{ pipeline.project_id }}</div>
          </div>
        </el-col>
        <el-col :span="6">
          <el-button v-if="pipeline.status === 'running'" type="danger" @click="handleCancel">
            取消流水线
          </el-button>
        </el-col>
      </el-row>
    </el-card>

    <!-- 6 阶段进度 -->
    <el-card title="流水线进度" style="margin-top: 16px">
      <el-steps :active="pipeline ? pipeline.current_stage - 1 : 0" finish-status="success">
        <el-step
          v-for="stage in stages"
          :key="stage.id"
          :title="getStageTitle(stage.stage_type)"
          :status="getStepStatus(stage.status)"
        >
          <template #description>
            <el-tag :type="getStatusColor(stage.status)">
              {{ getStatusText(stage.status) }}
            </el-tag>
          </template>
        </el-step>
      </el-steps>
    </el-card>

    <!-- 当前阶段详情 -->
    <el-card v-if="currentStage" :title="`当前阶段: ${getStageTitle(currentStage.stage_type)}`" style="margin-top: 16px">
      <el-descriptions :column="2">
        <el-descriptions-item label="阶段状态">
          <el-tag :type="getStatusColor(currentStage.status)">
            {{ getStatusText(currentStage.status) }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="执行节点">
          {{ currentStage.node_name || '未分配' }}
        </el-descriptions-item>
        <el-descriptions-item label="开始时间">
          {{ currentStage.started_at || '-' }}
        </el-descriptions-item>
        <el-descriptions-item label="容器 ID">
          {{ currentStage.container_id || '-' }}
        </el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 执行历史 -->
    <el-card title="执行历史" style="margin-top: 16px">
      <el-timeline>
        <el-timeline-item
          v-for="stage in completedStages"
          :key="stage.id"
          :color="stage.status === 'succeeded' ? 'green' : stage.status === 'failed' ? 'red' : 'blue'"
        >
          <p><strong>{{ getStageTitle(stage.stage_type) }}</strong></p>
          <p>
            状态: <el-tag :type="getStatusColor(stage.status)">{{ getStatusText(stage.status) }}</el-tag>
          </p>
          <p>开始: {{ stage.started_at }}</p>
          <p v-if="stage.finished_at">结束: {{ stage.finished_at }}</p>
          <el-alert
            v-if="stage.error_message"
            type="error"
            :title="stage.error_message"
            :closable="false"
            show-icon
          />
        </el-timeline-item>
      </el-timeline>
    </el-card>
  </div>
</template>

<script>
import axios from 'axios';

export default {
  name: 'PipelineDetailPage',
  data() {
    return {
      pipeline: null,
      stages: [],
      loading: true,
      timer: null
    };
  },
  computed: {
    currentStage() {
      return this.stages.find(s => s.status === 'running')
        || this.stages.find(s => s.stage_order === this.pipeline?.current_stage);
    },
    completedStages() {
      return this.stages.filter(s => s.started_at);
    }
  },
  created() {
    this.fetchPipelineDetail();
    this.fetchStages();
    this.loading = false;

    // 定时轮询
    this.timer = setInterval(() => {
      if (this.pipeline?.status === 'running') {
        this.fetchPipelineDetail();
        this.fetchStages();
      }
    }, 3000);
  },
  beforeDestroy() {
    if (this.timer) {
      clearInterval(this.timer);
    }
  },
  methods: {
    async fetchPipelineDetail() {
      try {
        const response = await axios.get(`/api/v1/pipelines/${this.$route.params.id}`);
        this.pipeline = response.data.data;
      } catch (error) {
        this.$message.error('获取流水线详情失败');
      }
    },

    async fetchStages() {
      try {
        const response = await axios.get(`/api/v1/pipelines/${this.$route.params.id}/stages`);
        this.stages = response.data.data;
      } catch (error) {
        this.$message.error('获取阶段列表失败');
      }
    },

    async handleCancel() {
      try {
        await axios.post(`/api/v1/pipelines/${this.$route.params.id}/cancel`);
        this.$message.success('流水线已取消');
        this.fetchPipelineDetail();
      } catch (error) {
        this.$message.error('取消失败');
      }
    },

    getStageTitle(stageType) {
      const titles = {
        'teacher_config': '教师模型配置',
        'dataset_build': '蒸馏数据构建',
        'teacher_infer': '教师推理',
        'data_govern': '数据治理',
        'student_train': '学生训练',
        'evaluate': '效果评估'
      };
      return titles[stageType] || stageType;
    },

    getStatusColor(status) {
      const colors = {
        'pending': 'info',
        'running': 'primary',
        'succeeded': 'success',
        'failed': 'danger',
        'canceled': 'warning'
      };
      return colors[status] || 'info';
    },

    getStatusText(status) {
      const texts = {
        'pending': '等待中',
        'scheduled': '已调度',
        'preparing': '准备中',
        'running': '运行中',
        'succeeded': '成功',
        'failed': '失败',
        'canceled': '已取消'
      };
      return texts[status] || status;
    },

    getStepStatus(status) {
      if (status === 'succeeded') return 'success';
      if (status === 'failed') return 'error';
      if (status === 'running') return 'process';
      return 'wait';
    }
  }
};
</script>

<style scoped>
.statistic {
  margin-bottom: 16px;
}
.statistic-title {
  font-size: 14px;
  color: #909399;
  margin-bottom: 8px;
}
.statistic-value {
  font-size: 24px;
  font-weight: bold;
}
</style>
```

---

## 总结

### 技术栈建议

- **框架**: Vue 2 + Vue Router
- **UI 组件库**: Element UI（中文友好、组件丰富）
- **状态管理**: Vuex
- **HTTP 客户端**: Axios
- **图表**: ECharts 或 Vue-ECharts（用于评估报告可视化）
- **WebSocket**: socket.io-client（用于实时日志推送，可选）

### 核心页面优先级

1. **P0 - 必须实现**：
   - 项目列表 + 详情页
   - 流水线列表 + 详情页（重点！）
   - 创建流水线向导

2. **P1 - 重要**：
   - 数据集管理
   - Worker 节点监控

3. **P2 - 可选增强**：
   - 实时日志推送（WebSocket）
   - 评估报告可视化
   - 用户权限管理

### 开发注意事项

1. **轮询策略**: 流水线详情页需要定时轮询（3-5秒），但要注意性能优化
2. **错误处理**: 所有 API 调用都要加 try-catch，并提供友好的错误提示
3. **加载状态**: 使用 Spin 组件显示加载状态，提升用户体验
4. **响应式设计**: 使用 Element UI 的 Grid 系统，支持移动端
5. **时间格式化**: 统一使用 dayjs 或 moment.js 处理时间
6. **数据刷新**: 可以配合 Vue 的响应式特性或使用 axios 拦截器来管理数据获取和缓存

---

如有任何问题，请随时联系后端团队！ 🚀
