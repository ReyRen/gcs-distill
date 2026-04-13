# 运行时逻辑技术文档

## 概述

运行时逻辑层是 GCS-Distill 的核心执行引擎，负责：
- 生成 EasyDistill 配置文件
- 管理数据清单和转换
- 执行数据治理
- 调度和监控容器执行

## 架构组件

```
runtime/
├── config_generator.go    # EasyDistill 配置生成器
├── manifest_manager.go    # 数据清单管理器
├── data_governor.go       # 数据治理器
└── stage_executor.go      # 阶段执行器（核心）
```

---

## 1. ConfigGenerator - 配置生成器

### 职责
为六个蒸馏阶段生成 EasyDistill 期望的 JSON 配置文件。

### 核心方法

#### GenerateTeacherInferConfig
生成教师模型推理配置（阶段3）

**输入**:
- Project: 项目信息（包含教师模型配置）
- RunID: 运行实例ID

**输出**: JSON 配置
```json
{
  "job_type": "kd_black_box_local",
  "dataset": {
    "instruction_path": "/workspace/data/seed/instructions.json",
    "labeled_path": "/workspace/data/generated/labeled.json"
  },
  "inference": {
    "temperature": 0.8,
    "max_new_tokens": 512
  },
  "models": {
    "teacher": "gpt-4"
  }
}
```

#### GenerateStudentTrainConfig
生成学生模型训练配置（阶段5）

**输入**:
- Project: 项目信息
- Pipeline: 流水线配置（包含训练超参数）
- RunID: 运行实例ID

**输出**: JSON 配置
```json
{
  "job_type": "kd_black_box_train_only",
  "dataset": {
    "instruction_path": "/workspace/data/filtered/train.json",
    "template": "chat_template/chat_template_kd.jinja"
  },
  "models": {
    "teacher": "gpt-4",
    "student": "qwen-7b"
  },
  "training": {
    "output_dir": "/workspace/models/checkpoints/",
    "num_train_epochs": 3,
    "per_device_train_batch_size": 4,
    "learning_rate": 2e-5,
    "save_steps": 1000
  }
}
```

#### GenerateEvaluateConfig
生成评估配置（阶段6）

### 路径管理

**工作空间结构**:
```
/shared/projects/{project_id}/runs/{run_id}/
├── configs/              # 配置文件
│   ├── teacher_infer.json
│   ├── student_train.json
│   └── evaluate.json
├── data/                 # 数据目录
│   ├── seed/            # 种子数据
│   ├── generated/       # 教师推理生成数据
│   └── filtered/        # 治理后数据
├── models/              # 模型输出
│   └── checkpoints/
├── logs/                # 日志
└── eval/                # 评估结果
```

---

## 2. ManifestManager - 清单管理器

### 职责
管理数据清单的创建、加载、保存和统计。

### 数据格式

#### Instruction（种子数据）
```json
{
  "instruction": "解释什么是机器学习",
  "input": "",
  "output": ""
}
```

#### LabeledData（教师推理输出）
```json
{
  "instruction": "解释什么是机器学习",
  "input": "",
  "output": "机器学习是人工智能的一个分支...",
  "teacher": "gpt-4"
}
```

#### TrainingData（治理后数据）
```json
{
  "instruction": "解释什么是机器学习",
  "input": "",
  "output": "机器学习是人工智能的一个分支...",
  "quality": 0.95
}
```

### 核心方法

#### CreateSeedManifest
将用户上传的原始数据转换为 JSONL 格式的种子数据清单。

**流程**:
1. 创建 `/workspace/data/seed/` 目录
2. 将 Instruction 数组写入 `instructions.json`
3. 每行一个 JSON 对象（JSONL 格式）

#### LoadLabeledData
加载教师模型推理生成的标注数据。

**输入**: projectID, runID
**输出**: []LabeledData

#### SaveFilteredData
保存数据治理后的训练集和测试集。

**流程**:
1. 创建 `/workspace/data/filtered/` 目录
2. 保存 `train.json`（90%数据）
3. 保存 `test.json`（10%数据）

#### GetManifestStats
统计各阶段数据量。

**返回**:
```go
{
  "seed": 1000,      // 种子数据
  "labeled": 980,    // 标注数据
  "train": 720,      // 训练数据
  "test": 80         // 测试数据
}
```

---

## 3. DataGovernor - 数据治理器

### 职责
对教师推理生成的数据进行质量过滤和处理。

### 治理流程

```
标注数据 (1000条)
   ↓
1. 空响应过滤 → 移除 50 条
   ↓
2. 长度校验   → 移除 30 条
   ↓
3. 去重       → 移除 120 条
   ↓
4. 质量评分   → 保留 800 条
   ↓
5. 划分       → 720 训练 + 80 测试
```

### 核心方法

#### FilterData
数据治理主流程。

**输入**: []LabeledData
**输出**: trainData, testData, stats

**统计信息**:
```go
{
  "total": 1000,
  "empty_removed": 50,
  "length_invalid": 30,
  "duplicates": 120,
  "filtered": 800,
  "train": 720,
  "test": 80
}
```

#### 过滤规则

**1. 空响应过滤**
- 完全为空的输出
- 包含"无法回答"、"抱歉"、"我不知道"等模式且长度<50

**2. 长度校验**
- 最小长度: 10 字符
- 最大长度: 4096 字符
- 使用 UTF-8 字符计数

**3. 去重**
- 基于 (instruction + output) 前100字符组合
- 使用 map 实现去重

**4. 质量评分**
简化版质量评分算法：
- 基础分: 1.0
- 长度惩罚: 太短(<50) -0.2，太长(>2000) -0.1
- 完整性奖励: 有 input 字段 +0.1
- 分数范围: [0, 1]

**5. 数据划分**
- 训练集: 90%
- 测试集: 10%

---

## 4. StageExecutor - 阶段执行器

### 职责
执行六个蒸馏阶段，协调各组件，调度 Worker 容器。

### 六阶段执行逻辑

#### 阶段1: Teacher Config（教师模型配置验证）
**类型**: 本地执行
**执行内容**:
1. 验证教师模型配置完整性
2. API 类型检查: base_url, api_key
3. Local 类型检查: model_path
4. 保存验证结果到 Manifest

**耗时**: < 1秒

---

#### 阶段2: Dataset Build（数据集构建）
**类型**: 本地执行
**执行内容**:
1. 创建工作空间目录结构
2. 从数据集加载原始数据
3. 转换为 EasyDistill 期望的 JSONL 格式
4. 保存种子数据清单

**目录创建**:
```
/workspace/
├── data/seed/
├── data/generated/
├── data/filtered/
├── configs/
├── logs/
├── models/checkpoints/
└── eval/
```

**耗时**: < 5秒

---

#### 阶段3: Teacher Infer（教师模型推理）
**类型**: Docker 容器执行
**执行内容**:
1. 生成 teacher_infer.json 配置
2. 调用 Worker 启动 EasyDistill 容器
3. 容器挂载工作空间
4. 监控容器执行状态
5. 等待容器完成

**Docker 命令**:
```bash
docker run --rm \
  -v /shared/projects/{project_id}/runs/{run_id}:/workspace \
  --gpus all \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/teacher_infer.json
```

**耗时**: 取决于数据量和模型速度（10分钟 - 数小时）

---

#### 阶段4: Data Govern（数据治理）
**类型**: 本地执行
**执行内容**:
1. 加载教师推理生成的 labeled.json
2. 执行数据治理流程
3. 保存 train.json 和 test.json

**耗时**: < 1分钟（万条数据）

---

#### 阶段5: Student Train（学生模型训练）
**类型**: Docker 容器执行
**执行内容**:
1. 生成 student_train.json 配置
2. 调用 Worker 启动 EasyDistill 训练容器
3. 监控训练进度
4. 等待训练完成

**Docker 命令**:
```bash
docker run --rm \
  -v /shared/projects/{project_id}/runs/{run_id}:/workspace \
  --gpus all \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/student_train.json
```

**耗时**: 数小时 - 数天（取决于模型大小和训练参数）

---

#### 阶段6: Evaluate（模型评估）
**类型**: Docker 容器执行
**执行内容**:
1. 生成 evaluate.json 配置
2. 调用 Worker 启动评估容器
3. 等待评估完成
4. 解析评估结果

**Docker 命令**:
```bash
docker run --rm \
  -v /shared/projects/{project_id}/runs/{run_id}:/workspace \
  --gpus device=0 \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/evaluate.json
```

**耗时**: 10分钟 - 1小时

---

### Worker 通信

#### runDockerContainer
通过 gRPC 调用 Worker 节点启动容器。

**gRPC 请求**:
```protobuf
message RunContainerRequest {
  string image = 1;
  repeated string command = 2;
  string work_dir = 3;
  repeated VolumeMount volume_mounts = 4;
  int32 gpu_count = 5;
  int32 memory_gb = 6;
  int32 cpu_cores = 7;
}
```

**返回**: containerID

#### waitForContainer
轮询容器状态直到完成。

**轮询间隔**: 5秒
**超时处理**: 由 context 控制

**状态检查**:
- `exited` + exitCode=0 → 成功
- `exited` + exitCode!=0 → 失败
- `error`, `failed` → 失败
- `running` → 继续等待

---

## 使用示例

### 完整流程示例代码

```go
// 创建执行器
executor := runtime.NewStageExecutor("/shared")

// 获取流水线和项目信息
pipeline := ... // 从数据库加载
project := ...
stage := ...

// 执行阶段
ctx := context.Background()
workerAddr := "worker-1:50051"

err := executor.ExecuteStage(ctx, stage, pipeline, project, workerAddr)
if err != nil {
    log.Fatalf("阶段执行失败: %v", err)
}

// 检查 Manifest
fmt.Printf("阶段清单: %+v\n", stage.Manifest)
```

---

## 错误处理

### 容器执行失败
- 非零退出码 → 返回错误
- 超时 → context 取消
- Worker 不可达 → gRPC 连接失败

### 数据文件缺失
- 种子数据不存在 → 阶段2失败
- 标注数据不存在 → 阶段4失败

### 配置生成失败
- 模型配置缺失 → 返回错误
- 训练配置缺失 → 返回错误

---

## 性能优化

### 大文件处理
- 使用 bufio.Scanner 流式读取 JSONL
- 避免一次性加载所有数据到内存

### 并发控制
- 每个阶段串行执行
- 但多个流水线可以并行执行

### 资源管理
- 容器自动清理（--rm）
- 日志文件定期归档

---

## 监控和日志

### 日志级别
- Info: 阶段开始/完成
- Warn: 数据过滤统计
- Error: 执行失败

### 日志示例
```
2026-04-13 10:00:00 INFO  开始执行阶段 stage_type=teacher_infer stage_id=xxx worker=worker-1:50051
2026-04-13 10:00:05 INFO  配置文件已生成 config=/workspace/configs/teacher_infer.json
2026-04-13 10:00:10 INFO  容器已启动 container_id=abc123
2026-04-13 10:15:30 INFO  教师模型推理完成
2026-04-13 10:15:35 INFO  数据治理完成 - 总数: 1000, 空响应: 50, 长度不符: 30, 重复: 120, 保留: 800 (训练: 720, 测试: 80)
```

---

## 扩展点

### 自定义数据治理规则
在 DataGovernor 中添加新的过滤器：
```go
func (g *DataGovernor) customFilter(data LabeledData) bool {
    // 自定义逻辑
    return true
}
```

### 支持新的阶段类型
在 StageExecutor.ExecuteStage 中添加新的 case：
```go
case types.StageCustom:
    return e.executeCustomStage(...)
```

### 支持其他蒸馏框架
修改配置生成器生成不同格式的配置。

---

## 总结

运行时逻辑层是 GCS-Distill 的执行引擎：
- ✅ **ConfigGenerator**: 为 EasyDistill 生成标准配置
- ✅ **ManifestManager**: 管理数据转换和清单
- ✅ **DataGovernor**: 智能数据治理和质量控制
- ✅ **StageExecutor**: 协调六阶段执行，调度容器

**代码量**: 1159 行
**文件数**: 4 个
**测试状态**: 待添加单元测试
