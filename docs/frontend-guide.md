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
  "data": { /* 具体数据 */ }
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
    "projects": [
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
    "datasets": [
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
    "gpu_count": 1,
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
  "message": "流水线启动成功",
  "data": {
    "id": "pipeline-uuid",
    "status": "scheduled",
    "current_stage": 1,
    "started_at": "2024-01-01T00:00:00Z"
  }
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
    "pipelines": [
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
  "data": {
    "stages": [
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
}
```

---

### 4. 资源管理 API

#### 4.1 获取 Worker 节点列表

**请求**

```http
GET /api/v1/resources/nodes
```

**响应**

```json
{
  "code": 200,
  "message": "获取节点列表成功",
  "data": {
    "nodes": [
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
    ],
    "total": 2
  }
}
```

#### 4.2 获取节点详情

**请求**

```http
GET /api/v1/resources/nodes/{node_name}
```

**响应**

```json
{
  "code": 200,
  "message": "获取节点成功",
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

### 5. 健康检查 API

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
  gpu_type?: string;         // "A100", "V100"
  memory_gb?: number;
  cpu_cores?: number;
}
```

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

**推荐组件库实现（Ant Design）**：

```tsx
import { Card, Button, Table, Input, Space } from 'antd';
import { PlusOutlined, SearchOutlined } from '@ant-design/icons';

const ProjectListPage = () => {
  return (
    <div>
      {/* 顶部操作栏 */}
      <Space style={{ marginBottom: 16 }}>
        <Input
          placeholder="搜索项目名称"
          prefix={<SearchOutlined />}
          style={{ width: 300 }}
        />
        <Button type="primary" icon={<PlusOutlined />}>
          创建项目
        </Button>
      </Space>

      {/* 项目卡片列表 */}
      <Row gutter={[16, 16]}>
        {projects.map(project => (
          <Col span={8} key={project.id}>
            <Card
              title={project.name}
              extra={<Button type="link">查看</Button>}
              actions={[
                <Button type="link">编辑</Button>,
                <Button type="link" danger>删除</Button>
              ]}
            >
              <p>{project.description}</p>
              <p>
                教师: {project.teacher_model_config.model_name} →
                学生: {project.student_model_config.model_name}
              </p>
              <p>创建时间: {formatDate(project.created_at)}</p>
            </Card>
          </Col>
        ))}
      </Row>
    </div>
  );
};
```

#### 2.2 创建/编辑项目页

**布局要点**：
- 表单式布局，分为几个区域：
  1. 基本信息（项目名称、描述、业务场景）
  2. 教师模型配置（提供者类型、模型名称、API 配置）
  3. 学生模型配置（模型名称、模型路径）
  4. 评估配置（评估指标、测试集比例）

**表单示例**：

```tsx
import { Form, Input, Select, InputNumber, Switch } from 'antd';

const ProjectForm = () => {
  return (
    <Form layout="vertical">
      {/* 基本信息 */}
      <Form.Item label="项目名称" name="name" required>
        <Input placeholder="例如：客服问答蒸馏" />
      </Form.Item>

      <Form.Item label="项目描述" name="description">
        <Input.TextArea rows={3} />
      </Form.Item>

      {/* 教师模型配置 */}
      <Divider>教师模型配置</Divider>

      <Form.Item label="提供者类型" name={['teacher_model_config', 'provider_type']}>
        <Select>
          <Option value="api">API 型（OpenAI/Claude）</Option>
          <Option value="local">本地模型</Option>
        </Select>
      </Form.Item>

      <Form.Item label="模型名称" name={['teacher_model_config', 'model_name']}>
        <Input placeholder="例如：Qwen/Qwen2.5-7B-Instruct" />
      </Form.Item>

      <Form.Item label="API 端点" name={['teacher_model_config', 'endpoint']}>
        <Input placeholder="https://api.openai.com/v1/chat/completions" />
      </Form.Item>

      <Form.Item label="Temperature" name={['teacher_model_config', 'temperature']}>
        <InputNumber min={0} max={2} step={0.1} />
      </Form.Item>

      {/* 学生模型配置 */}
      <Divider>学生模型配置</Divider>

      <Form.Item label="模型名称" name={['student_model_config', 'model_name']}>
        <Input placeholder="例如：Qwen/Qwen2.5-0.5B-Instruct" />
      </Form.Item>

      {/* 评估配置 */}
      <Divider>评估配置</Divider>

      <Form.Item label="评估指标" name={['evaluation_config', 'metrics']}>
        <Select mode="multiple">
          <Option value="bleu">BLEU</Option>
          <Option value="rouge">ROUGE</Option>
          <Option value="accuracy">准确率</Option>
        </Select>
      </Form.Item>
    </Form>
  );
};
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

```tsx
import { Tabs, Descriptions, Statistic, Row, Col } from 'antd';

const ProjectDetailPage = () => {
  return (
    <div>
      {/* 项目头部信息 */}
      <Card>
        <Row gutter={16}>
          <Col span={18}>
            <h2>{project.name}</h2>
            <p>{project.description}</p>
          </Col>
          <Col span={6}>
            <Statistic title="流水线总数" value={10} />
            <Statistic title="成功次数" value={8} />
          </Col>
        </Row>
      </Card>

      {/* Tab 导航 */}
      <Tabs defaultActiveKey="overview">
        <TabPane tab="概览" key="overview">
          <Descriptions>
            <Item label="教师模型">{project.teacher_model_config.model_name}</Item>
            <Item label="学生模型">{project.student_model_config.model_name}</Item>
            <Item label="业务场景">{project.business_scenario}</Item>
          </Descriptions>
        </TabPane>

        <TabPane tab="数据集" key="datasets">
          {/* 数据集列表 */}
        </TabPane>

        <TabPane tab="流水线" key="pipelines">
          {/* 流水线列表 */}
        </TabPane>

        <TabPane tab="设置" key="settings">
          {/* 项目编辑表单 */}
        </TabPane>
      </Tabs>
    </div>
  );
};
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

```tsx
import { Upload, Table } from 'antd';
import { InboxOutlined } from '@ant-design/icons';

const DatasetUpload = () => {
  return (
    <div>
      <Upload.Dragger
        accept=".json,.jsonl,.csv"
        beforeUpload={handleUpload}
      >
        <p className="ant-upload-drag-icon">
          <InboxOutlined />
        </p>
        <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
        <p className="ant-upload-hint">
          支持 .json, .jsonl, .csv 格式
        </p>
      </Upload.Dragger>

      {/* 数据预览 */}
      {previewData && (
        <Card title="数据预览（前 10 条）" style={{ marginTop: 16 }}>
          <Table
            dataSource={previewData}
            columns={previewColumns}
            pagination={false}
          />
        </Card>
      )}
    </div>
  );
};
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

```tsx
import { Tag } from 'antd';

const statusConfig = {
  pending: { color: 'default', text: '等待中' },
  scheduled: { color: 'processing', text: '已调度' },
  preparing: { color: 'processing', text: '准备中' },
  running: { color: 'processing', text: '运行中' },
  succeeded: { color: 'success', text: '成功' },
  failed: { color: 'error', text: '失败' },
  canceled: { color: 'warning', text: '已取消' }
};

const PipelineStatusTag = ({ status }) => {
  const config = statusConfig[status];
  return <Tag color={config.color}>{config.text}</Tag>;
};
```

#### 4.2 创建流水线页

**布局要点**：
- Step 1: 选择项目和数据集
- Step 2: 配置训练参数
- Step 3: 配置资源需求
- Step 4: 确认并创建

**Steps 组件示例**：

```tsx
import { Steps, Form, Select, InputNumber } from 'antd';

const CreatePipelineWizard = () => {
  const [current, setCurrent] = useState(0);

  const steps = [
    {
      title: '选择项目和数据集',
      content: (
        <Form layout="vertical">
          <Form.Item label="项目" name="project_id">
            <Select placeholder="选择项目">
              {projects.map(p => (
                <Option key={p.id} value={p.id}>{p.name}</Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label="数据集" name="dataset_id">
            <Select placeholder="选择数据集">
              {datasets.map(d => (
                <Option key={d.id} value={d.id}>{d.name}</Option>
              ))}
            </Select>
          </Form.Item>
        </Form>
      )
    },
    {
      title: '训练参数',
      content: (
        <Form layout="vertical">
          <Form.Item label="训练轮数" name={['training_config', 'num_train_epochs']}>
            <InputNumber min={1} max={100} />
          </Form.Item>
          <Form.Item label="学习率" name={['training_config', 'learning_rate']}>
            <InputNumber min={0} max={1} step={0.00001} />
          </Form.Item>
          {/* 更多训练参数... */}
        </Form>
      )
    },
    {
      title: '资源需求',
      content: (
        <Form layout="vertical">
          <Form.Item label="GPU 数量" name={['resource_request', 'gpu_count']}>
            <InputNumber min={0} max={8} />
          </Form.Item>
          <Form.Item label="内存 (GB)" name={['resource_request', 'memory_gb']}>
            <InputNumber min={8} max={512} />
          </Form.Item>
        </Form>
      )
    }
  ];

  return (
    <div>
      <Steps current={current}>
        {steps.map(item => (
          <Step key={item.title} title={item.title} />
        ))}
      </Steps>

      <div style={{ marginTop: 24 }}>
        {steps[current].content}
      </div>

      <div style={{ marginTop: 24 }}>
        {current > 0 && (
          <Button onClick={() => setCurrent(current - 1)}>上一步</Button>
        )}
        {current < steps.length - 1 && (
          <Button type="primary" onClick={() => setCurrent(current + 1)}>
            下一步
          </Button>
        )}
        {current === steps.length - 1 && (
          <Button type="primary" onClick={handleSubmit}>
            创建流水线
          </Button>
        )}
      </div>
    </div>
  );
};
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

```tsx
import { Steps, Card, Timeline, Badge, Progress } from 'antd';

const PipelineDetailPage = () => {
  const [pipeline, setPipeline] = useState(null);
  const [stages, setStages] = useState([]);

  // 定时刷新流水线状态（轮询）
  useEffect(() => {
    const timer = setInterval(() => {
      fetchPipelineDetail();
      fetchStages();
    }, 3000); // 每 3 秒刷新
    return () => clearInterval(timer);
  }, []);

  return (
    <div>
      {/* 流水线头部信息 */}
      <Card>
        <Row gutter={16}>
          <Col span={6}>
            <Statistic title="流水线状态" value={pipeline.status} />
          </Col>
          <Col span={6}>
            <Statistic title="当前阶段" value={`${pipeline.current_stage}/6`} />
          </Col>
          <Col span={6}>
            <Statistic
              title="已运行时间"
              value={calculateDuration(pipeline.started_at)}
            />
          </Col>
          <Col span={6}>
            <Button danger onClick={handleCancel}>取消流水线</Button>
          </Col>
        </Row>
      </Card>

      {/* 6 阶段进度条 */}
      <Card title="流水线进度" style={{ marginTop: 16 }}>
        <Steps current={pipeline.current_stage - 1}>
          {stages.map((stage, index) => (
            <Step
              key={stage.id}
              title={getStageTitle(stage.stage_type)}
              description={
                <div>
                  <PipelineStatusTag status={stage.status} />
                  {stage.finished_at && (
                    <div>耗时: {calculateDuration(stage.started_at, stage.finished_at)}</div>
                  )}
                </div>
              }
              icon={getStageIcon(stage.status)}
            />
          ))}
        </Steps>
      </Card>

      {/* 当前阶段详情 */}
      {currentStage && (
        <Card title={`阶段 ${currentStage.stage_order}: ${getStageTitle(currentStage.stage_type)}`} style={{ marginTop: 16 }}>
          <Descriptions column={2}>
            <Item label="状态">
              <PipelineStatusTag status={currentStage.status} />
            </Item>
            <Item label="执行节点">{currentStage.node_name}</Item>
            <Item label="开始时间">{formatDateTime(currentStage.started_at)}</Item>
            <Item label="容器 ID">{currentStage.container_id}</Item>
          </Descriptions>

          {/* 实时日志 */}
          <Divider>实时日志</Divider>
          <div
            style={{
              background: '#000',
              color: '#0f0',
              padding: 16,
              height: 400,
              overflow: 'auto',
              fontFamily: 'monospace'
            }}
          >
            {logs.map((log, i) => (
              <div key={i}>{log}</div>
            ))}
          </div>
        </Card>
      )}

      {/* 阶段历史记录 */}
      <Card title="阶段执行历史" style={{ marginTop: 16 }}>
        <Timeline>
          {stages.filter(s => s.status !== 'pending').map(stage => (
            <Timeline.Item
              key={stage.id}
              color={stage.status === 'succeeded' ? 'green' : stage.status === 'failed' ? 'red' : 'blue'}
            >
              <p>
                <strong>{getStageTitle(stage.stage_type)}</strong> -
                <PipelineStatusTag status={stage.status} />
              </p>
              <p>开始: {formatDateTime(stage.started_at)}</p>
              {stage.finished_at && (
                <p>结束: {formatDateTime(stage.finished_at)}</p>
              )}
              {stage.error_message && (
                <Alert type="error" message={stage.error_message} />
              )}
            </Timeline.Item>
          ))}
        </Timeline>
      </Card>
    </div>
  );
};

// 辅助函数
const getStageTitle = (stageType) => {
  const titles = {
    'teacher_config': '1. 教师模型配置',
    'dataset_build': '2. 蒸馏数据构建',
    'teacher_infer': '3. 教师推理',
    'data_govern': '4. 数据治理',
    'student_train': '5. 学生训练',
    'evaluate': '6. 效果评估'
  };
  return titles[stageType] || stageType;
};

const getStageIcon = (status) => {
  if (status === 'succeeded') return <CheckCircleOutlined />;
  if (status === 'failed') return <CloseCircleOutlined />;
  if (status === 'running') return <LoadingOutlined />;
  return <ClockCircleOutlined />;
};
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

```tsx
import { Card, Progress, Badge, Row, Col } from 'antd';

const WorkerNodeCard = ({ node }) => {
  const gpuUsage = ((node.total_gpu - node.available_gpu) / node.total_gpu) * 100;

  return (
    <Card
      title={
        <span>
          <Badge status={node.status === 'online' ? 'success' : 'error'} />
          {node.node_name}
        </span>
      }
      extra={<Tag>{node.status}</Tag>}
    >
      <p>地址: {node.node_addr}</p>

      <Divider />

      <div>
        <p>GPU 使用情况</p>
        <Progress
          percent={gpuUsage}
          format={() => `${node.total_gpu - node.available_gpu}/${node.total_gpu}`}
          status={gpuUsage > 80 ? 'exception' : 'normal'}
        />
      </div>

      <div style={{ marginTop: 16 }}>
        <p>资源配置</p>
        <Row>
          <Col span={12}>总内存: {node.total_memory_gb} GB</Col>
          <Col span={12}>CPU 核心: {node.total_cpu}</Col>
        </Row>
      </div>

      <div style={{ marginTop: 16 }}>
        <p>最后心跳: {formatRelativeTime(node.last_heartbeat)}</p>
      </div>
    </Card>
  );
};

const WorkerNodesPage = () => {
  return (
    <Row gutter={[16, 16]}>
      {nodes.map(node => (
        <Col span={8} key={node.node_name}>
          <WorkerNodeCard node={node} />
        </Col>
      ))}
    </Row>
  );
};
```

---

## 状态管理建议

### 使用 Redux Toolkit 或 Zustand

**推荐 Zustand**（更轻量）：

```typescript
// stores/projectStore.ts
import create from 'zustand';

interface ProjectStore {
  projects: Project[];
  currentProject: Project | null;
  loading: boolean;

  fetchProjects: () => Promise<void>;
  fetchProject: (id: string) => Promise<void>;
  createProject: (project: Partial<Project>) => Promise<void>;
  updateProject: (id: string, updates: Partial<Project>) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
}

export const useProjectStore = create<ProjectStore>((set, get) => ({
  projects: [],
  currentProject: null,
  loading: false,

  fetchProjects: async () => {
    set({ loading: true });
    try {
      const response = await fetch('/api/v1/projects');
      const data = await response.json();
      set({ projects: data.data.projects, loading: false });
    } catch (error) {
      set({ loading: false });
      message.error('获取项目列表失败');
    }
  },

  createProject: async (project) => {
    const response = await fetch('/api/v1/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(project)
    });
    const data = await response.json();
    if (data.code === 200) {
      message.success('项目创建成功');
      get().fetchProjects();
    }
  },

  // ... 其他方法
}));
```

**在组件中使用**：

```tsx
const ProjectListPage = () => {
  const { projects, loading, fetchProjects } = useProjectStore();

  useEffect(() => {
    fetchProjects();
  }, []);

  return (
    <Table
      dataSource={projects}
      loading={loading}
      columns={columns}
    />
  );
};
```

---

## 开发示例

### 完整的流水线详情页实现

```tsx
// pages/PipelineDetailPage.tsx
import React, { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import {
  Card, Steps, Descriptions, Button, Timeline,
  Alert, Spin, Statistic, Row, Col, Tag, message
} from 'antd';
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  LoadingOutlined,
  ClockCircleOutlined
} from '@ant-design/icons';
import axios from 'axios';

const { Step } = Steps;
const { Item } = Descriptions;

const PipelineDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [pipeline, setPipeline] = useState<any>(null);
  const [stages, setStages] = useState<any[]>([]);
  const [logs, setLogs] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);

  // 获取流水线详情
  const fetchPipelineDetail = async () => {
    try {
      const response = await axios.get(`/api/v1/pipelines/${id}`);
      setPipeline(response.data.data);
    } catch (error) {
      message.error('获取流水线详情失败');
    }
  };

  // 获取阶段列表
  const fetchStages = async () => {
    try {
      const response = await axios.get(`/api/v1/pipelines/${id}/stages`);
      setStages(response.data.data.stages);
    } catch (error) {
      message.error('获取阶段列表失败');
    }
  };

  // 轮询更新
  useEffect(() => {
    fetchPipelineDetail();
    fetchStages();
    setLoading(false);

    const timer = setInterval(() => {
      if (pipeline?.status === 'running') {
        fetchPipelineDetail();
        fetchStages();
      }
    }, 3000);

    return () => clearInterval(timer);
  }, [id]);

  // 取消流水线
  const handleCancel = async () => {
    try {
      await axios.post(`/api/v1/pipelines/${id}/cancel`);
      message.success('流水线已取消');
      fetchPipelineDetail();
    } catch (error) {
      message.error('取消失败');
    }
  };

  if (loading) return <Spin size="large" />;
  if (!pipeline) return <Alert type="error" message="流水线不存在" />;

  const currentStage = stages.find(s => s.status === 'running');

  return (
    <div style={{ padding: 24 }}>
      {/* 流水线头部 */}
      <Card>
        <Row gutter={16}>
          <Col span={6}>
            <Statistic
              title="流水线状态"
              value={pipeline.status}
              valueStyle={{ color: pipeline.status === 'succeeded' ? '#3f8600' : '#cf1322' }}
            />
          </Col>
          <Col span={6}>
            <Statistic title="当前阶段" value={`${pipeline.current_stage}/6`} />
          </Col>
          <Col span={6}>
            <Statistic title="项目名称" value={pipeline.project_id} />
          </Col>
          <Col span={6}>
            {pipeline.status === 'running' && (
              <Button danger onClick={handleCancel}>取消流水线</Button>
            )}
          </Col>
        </Row>
      </Card>

      {/* 6 阶段进度 */}
      <Card title="流水线进度" style={{ marginTop: 16 }}>
        <Steps current={pipeline.current_stage - 1}>
          {stages.map((stage) => (
            <Step
              key={stage.id}
              title={getStageTitle(stage.stage_type)}
              status={getStepStatus(stage.status)}
              icon={getStageIcon(stage.status)}
              description={
                <div>
                  <Tag color={getStatusColor(stage.status)}>
                    {getStatusText(stage.status)}
                  </Tag>
                </div>
              }
            />
          ))}
        </Steps>
      </Card>

      {/* 当前阶段详情 */}
      {currentStage && (
        <Card title={`当前阶段: ${getStageTitle(currentStage.stage_type)}`} style={{ marginTop: 16 }}>
          <Descriptions column={2}>
            <Item label="阶段状态">
              <Tag color={getStatusColor(currentStage.status)}>
                {getStatusText(currentStage.status)}
              </Tag>
            </Item>
            <Item label="执行节点">{currentStage.node_name || '未分配'}</Item>
            <Item label="开始时间">{currentStage.started_at || '-'}</Item>
            <Item label="容器 ID">{currentStage.container_id || '-'}</Item>
          </Descriptions>
        </Card>
      )}

      {/* 执行历史 */}
      <Card title="执行历史" style={{ marginTop: 16 }}>
        <Timeline>
          {stages.filter(s => s.started_at).map(stage => (
            <Timeline.Item
              key={stage.id}
              color={stage.status === 'succeeded' ? 'green' : stage.status === 'failed' ? 'red' : 'blue'}
              dot={getStageIcon(stage.status)}
            >
              <p><strong>{getStageTitle(stage.stage_type)}</strong></p>
              <p>状态: <Tag color={getStatusColor(stage.status)}>{getStatusText(stage.status)}</Tag></p>
              <p>开始: {stage.started_at}</p>
              {stage.finished_at && <p>结束: {stage.finished_at}</p>}
              {stage.error_message && (
                <Alert type="error" message={stage.error_message} showIcon />
              )}
            </Timeline.Item>
          ))}
        </Timeline>
      </Card>
    </div>
  );
};

// 辅助函数
const getStageTitle = (stageType: string) => {
  const titles: Record<string, string> = {
    'teacher_config': '教师模型配置',
    'dataset_build': '蒸馏数据构建',
    'teacher_infer': '教师推理',
    'data_govern': '数据治理',
    'student_train': '学生训练',
    'evaluate': '效果评估'
  };
  return titles[stageType] || stageType;
};

const getStatusColor = (status: string) => {
  const colors: Record<string, string> = {
    'pending': 'default',
    'running': 'processing',
    'succeeded': 'success',
    'failed': 'error',
    'canceled': 'warning'
  };
  return colors[status] || 'default';
};

const getStatusText = (status: string) => {
  const texts: Record<string, string> = {
    'pending': '等待中',
    'scheduled': '已调度',
    'preparing': '准备中',
    'running': '运行中',
    'succeeded': '成功',
    'failed': '失败',
    'canceled': '已取消'
  };
  return texts[status] || status;
};

const getStepStatus = (status: string) => {
  if (status === 'succeeded') return 'finish';
  if (status === 'failed') return 'error';
  if (status === 'running') return 'process';
  return 'wait';
};

const getStageIcon = (status: string) => {
  if (status === 'succeeded') return <CheckCircleOutlined />;
  if (status === 'failed') return <CloseCircleOutlined />;
  if (status === 'running') return <LoadingOutlined />;
  return <ClockCircleOutlined />;
};

export default PipelineDetailPage;
```

---

## 总结

### 技术栈建议

- **UI 框架**: Ant Design（中文友好、组件丰富）
- **路由**: React Router v6
- **状态管理**: Zustand 或 Redux Toolkit
- **HTTP 客户端**: Axios
- **图表**: ECharts 或 Recharts（用于评估报告可视化）
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
4. **响应式设计**: 使用 Ant Design 的 Grid 系统，支持移动端
5. **时间格式化**: 统一使用 dayjs 或 moment.js 处理时间
6. **数据刷新**: 考虑使用 SWR 或 React Query 来管理数据获取和缓存

---

如有任何问题，请随时联系后端团队！ 🚀
