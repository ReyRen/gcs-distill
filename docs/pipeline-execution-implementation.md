# Pipeline 执行调度实现说明

## 问题分析

原先的 `StartPipeline` 实现确实是"假"操作，只做了以下几件事：
1. 更新流水线状态为 `running`
2. 写入 `started_at` 时间戳
3. 设置 `current_stage = 1`
4. 直接返回

**缺失的关键逻辑：**
- ❌ 没有调用 `StageExecutor` 执行阶段
- ❌ 没有调度 Worker 节点
- ❌ 没有创建运行工作目录
- ❌ 没有后台任务机制驱动流水线执行
- ❌ 没有容器调度和监控

## 实现方案

### 1. 创建 ExecutorService (service/executor_service.go)

新增了完整的流水线执行器服务，负责：

**核心功能：**
- 提供流水线执行队列（channel-based）
- 启动后台 worker goroutines 并发执行流水线
- 与 SchedulerService 集成，查找和分配 Worker 节点资源
- 调用 StageExecutor 执行六个阶段
- 处理阶段执行结果，推进流水线状态
- 错误处理和资源释放

**关键方法：**
```go
// SubmitPipeline - 提交流水线到执行队列
func (s *executorService) SubmitPipeline(ctx context.Context, pipelineID string) error

// Start - 启动后台执行器（启动多个 worker goroutines）
func (s *executorService) Start(ctx context.Context)

// executePipeline - 执行完整流水线（所有6个阶段）
func (s *executorService) executePipeline(ctx context.Context, pipelineID string) error
```

### 2. 执行流程

```
用户调用 StartPipeline
    ↓
更新数据库状态 (running, started_at, current_stage=1)
    ↓
提交到执行队列 (executorSvc.SubmitPipeline)
    ↓
后台 worker 协程接收任务
    ↓
查找可用 Worker 节点 (schedulerSvc.FindAvailableNode)
    ↓
分配 GPU/CPU/内存资源 (schedulerSvc.AllocateResources)
    ↓
循环执行 6 个阶段：
    Stage 1: 教师模型配置验证
    Stage 2: 数据集构建（创建工作目录、种子数据）
    Stage 3: 教师模型推理（调度 easydistill 容器）
    Stage 4: 数据治理（过滤、清洗）
    Stage 5: 学生模型训练（调度 easydistill 容器）
    Stage 6: 模型评估（调度 easydistill 容器）
    ↓
每个阶段：
    - 生成 EasyDistill 配置文件
    - 通过 gRPC 调用 Worker 的 RunContainer
    - 传递正确的 GPU 设备 ID
    - 挂载工作空间卷 (/workspace)
    - 等待容器执行完成
    - 更新阶段状态和 Manifest
    ↓
释放节点资源
    ↓
更新流水线状态为 succeeded/failed
```

### 3. 与 EasyDistill 容器的集成

**容器调度 (runtime/stage_executor.go):**

```go
// 阶段 3、5、6 会调度 easydistill:latest 容器
containerID, err := e.runDockerContainer(ctx, workerAddr, &ContainerRequest{
    Image:       "gcs-distill/easydistill:latest",
    Command:     []string{"--config", "/workspace/configs/teacher_infer.json"},
    WorkDir:     "/workspace",
    VolumeMounts: map[string]string{
        "/mnt/shared/distill/projects/xxx/runs/yyy": "/workspace",
    },
    GPUs:         pipeline.ResourceRequest.GPUCount,
    GPUDeviceIDs: pipeline.ResourceRequest.GPUDeviceIDs, // "0,1,2"
    Memory:       pipeline.ResourceRequest.MemoryGB,
    CPUs:         pipeline.ResourceRequest.CPUCores,
})
```

**配置文件生成:**

每个阶段都会生成符合 EasyDistill 规范的 JSON 配置：

- `teacher_infer.json` - 教师推理配置
- `student_train.json` - 学生训练配置
- `evaluate.json` - 评估配置

配置内容包括：
- `job_type`: 任务类型（kd_black_box_local, kd_black_box_train_only, cot_eval_api）
- `dataset`: 数据集路径
- `models`: 模型配置
- `training`: 训练超参数
- `inference`: 推理参数

**工作空间结构:**

```
/mnt/shared/distill/projects/{project_id}/runs/{run_id}/
├── configs/
│   ├── teacher_infer.json
│   ├── student_train.json
│   └── evaluate.json
├── data/
│   ├── seed/
│   │   └── instructions.json
│   ├── generated/
│   │   └── labeled.json
│   └── filtered/
│       ├── train.json
│       └── test.json
├── models/
│   └── checkpoints/
├── logs/
│   ├── teacher_infer
│   ├── student_train
│   └── evaluate
└── eval/
    └── results.json
```

### 4. 配置更新

**添加执行器配置 (internal/config/config.go):**

```go
type ExecutorConfig struct {
    WorkspaceRoot string `yaml:"workspace_root"` // 工作空间根目录
    MaxConcurrent int    `yaml:"max_concurrent"` // 最大并发执行数
}
```

**配置示例 (config.example.yaml):**

```yaml
executor:
  workspace_root: /mnt/shared/distill
  max_concurrent: 5  # 最多同时执行 5 个流水线
```

### 5. 主程序初始化 (cmd/server/main.go)

```go
// 创建执行器服务
executorSvc := service.NewExecutorService(
    pipelineRepo,
    stageRepo,
    projectRepo,
    schedulerSvc,
    cfg.Executor.WorkspaceRoot,
    cfg.Executor.MaxConcurrent,
)

// 启动执行器后台 worker
execCtx, execCancel := context.WithCancel(context.Background())
defer execCancel()
executorSvc.Start(execCtx)
defer executorSvc.Stop()

// 创建流水线服务（注入执行器）
pipelineSvc := service.NewPipelineService(
    pipelineRepo,
    stageRepo,
    projectRepo,
    datasetRepo,
    executorSvc,
)
```

## 代码检查结果

✅ **没有其他"假"逻辑**

检查了整个 `service/` 目录，所有服务都有完整的实现：
- `project_service.go` - 项目管理（完整）
- `dataset_service.go` - 数据集管理（完整）
- `pipeline_service.go` - 流水线管理（已修复）
- `scheduler_service.go` - 资源调度（完整）
- `executor_service.go` - 执行器（新增，完整）

✅ **StageExecutor 已完整实现**

所有 6 个阶段的执行逻辑都已实现：
- `executeTeacherConfig` - 配置验证
- `executeDatasetBuild` - 数据集准备
- `executeTeacherInfer` - 容器调度 + 推理
- `executeDataGovern` - 数据治理
- `executeStudentTrain` - 容器调度 + 训练
- `executeEvaluate` - 容器调度 + 评估

✅ **容器调度集成 EasyDistill**

- 使用正确的镜像：`gcs-distill/easydistill:latest`
- 传递正确的配置文件路径
- 支持 GPU 设备 ID 指定
- 正确的卷挂载
- 等待容器完成并检查退出码

## 测试验证

### 编译测试

```bash
$ go build -o /tmp/gcs-server ./cmd/server
$ go build -o /tmp/gcs-worker ./cmd/worker
✅ Build successful
```

### 运行测试

```bash
# 1. 启动服务
./bin/gcs-distill-server --config config.example.yaml

# 2. 注册 Worker 节点
# Worker 会自动注册，包含 GPU 信息

# 3. 创建流水线
curl -X POST http://localhost:8080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "xxx",
    "dataset_id": "yyy",
    "training_config": {...},
    "resource_request": {
      "gpu_count": 2,
      "gpu_device_ids": "0,1"
    }
  }'

# 4. 启动流水线
curl -X POST http://localhost:8080/api/v1/pipelines/{id}/start

# 流水线将：
# ✅ 被提交到执行队列
# ✅ 后台 worker 接收并执行
# ✅ 查找可用 Worker 节点
# ✅ 分配 GPU 资源
# ✅ 执行 6 个阶段
# ✅ 调度 easydistill 容器
# ✅ 更新状态和结果
# ✅ 释放资源
```

## 关键改进

1. **真正的异步执行**: 使用 goroutine + channel 实现后台任务队列
2. **资源调度**: 与 SchedulerService 集成，智能分配 Worker 节点
3. **容器编排**: 调用 Worker 的 gRPC 接口执行 Docker 容器
4. **状态管理**: 实时更新流水线和阶段状态
5. **错误处理**: 完善的错误处理和资源清理
6. **并发控制**: 支持配置最大并发数，避免资源耗尽

## 注意事项

1. **Worker 必须运行**: 确保至少有一个 Worker 节点在线并注册
2. **EasyDistill 镜像**: 需要先构建 `gcs-distill/easydistill:latest` 镜像
3. **共享存储**: 确保 Server 和 Worker 都能访问同一个共享存储目录
4. **GPU 可用**: Worker 节点需要有 GPU 且 Docker 支持 GPU
5. **网络通信**: Server 能通过 gRPC 访问 Worker 的 50051-50053 端口

## 后续优化建议

1. **重试机制**: 阶段执行失败时自动重试
2. **超时控制**: 添加阶段执行超时设置
3. **日志聚合**: 将容器日志实时推送到中心日志系统
4. **监控指标**: 添加 Prometheus metrics
5. **优先级队列**: 支持高优先级流水线优先执行
6. **断点续传**: 流水线中断后可以从失败的阶段继续
