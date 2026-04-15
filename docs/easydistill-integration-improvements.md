# EasyDistill 容器集成改进文档

本文档描述了 GCS-Distill 项目与 EasyDistill 容器集成的改进。

## 改进概述

### 1. GPU 设备指定支持

**问题**: 原实现只支持使用所有 GPU（`--gpus all`），无法指定特定的 GPU 设备。

**解决方案**:
- 在 `proto/worker.proto` 中添加 `gpu_device_ids` 字段
- 在 `internal/docker/client.go` 中添加 `GPUDeviceIDs` 字段和逻辑
- 支持三种 GPU 配置模式：
  1. 指定设备 ID：`GPUDeviceIDs = "0,1"` 使用 GPU 0 和 1
  2. 指定数量：`GPUCount = 2` 使用任意 2 个 GPU
  3. 使用所有：`GPUCount = -1` 使用所有可用 GPU

**使用示例**:
```go
// 指定使用 GPU 0 和 1
containerReq := &ContainerRequest{
    Image: "gcs-distill/easydistill:latest",
    GPUDeviceIDs: "0,1",
    // ...
}

// 使用任意 2 个 GPU
containerReq := &ContainerRequest{
    Image: "gcs-distill/easydistill:latest",
    GPUCount: 2,
    // ...
}

// 使用所有 GPU
containerReq := &ContainerRequest{
    Image: "gcs-distill/easydistill:latest",
    GPUCount: -1,
    // ...
}
```

### 2. 日志持久化机制

**问题**: 容器日志只能通过 Docker API 获取，容器删除后日志丢失。

**解决方案**:
- EasyDistill 配置中添加 `logging.log_file` 配置项
- 日志直接写入 `/workspace/logs/` 目录
- 添加 `ReadLogFile` 和 `TailLogFile` 方法从文件读取日志
- 日志文件持久化在共享存储中，容器删除后仍可访问

**工作空间结构**:
```
/workspace/
├── configs/              # 配置文件
├── data/                 # 数据目录
├── models/              # 模型输出
└── logs/                # 日志文件（持久化）
    ├── teacher_infer.log
    ├── student_train.log
    └── evaluate.log
```

**API 使用**:
```go
// 读取完整日志（需要传入数据集仓库）
datasetRepo := postgres.NewDatasetRepository(db)
executor := runtime.NewStageExecutor("/shared", datasetRepo)
logs, err := executor.ReadLogFile(projectID, runID, "teacher_infer")

// 读取最后 100 行
logs, err := executor.TailLogFile(projectID, runID, "teacher_infer", 100)
```

### 3. 镜像构建和测试

**改进内容**:
1. 修复 Makefile 中 `docker-build` 的构建上下文路径
2. 添加 `docker-test` 目标用于快速测试镜像
3. 创建完整的集成测试脚本

**Makefile 命令**:
```bash
# 构建 EasyDistill 镜像
make docker-build

# 测试镜像基本功能
make docker-test

# 运行集成测试
make test-integration

# 运行端到端测试
make test-e2e

# 运行所有测试
make test-all
```

### 4. 集成测试脚本

创建了两个测试脚本：

#### a. `test_easydistill.sh` - 容器基础功能测试

**测试内容**:
- ✅ 镜像存在性检查
- ✅ 镜像基本功能（--help）
- ✅ 工作空间卷挂载
- ✅ 日志文件写入到 `/workspace/logs/`
- ✅ 模型输出目录可访问性
- ✅ GPU 支持（如果可用）
- ✅ EasyDistill 命令可用性

**运行方式**:
```bash
./tests/integration/test_easydistill.sh
```

#### b. `test_e2e_workflow.sh` - 端到端工作流测试

**测试内容**:
- ✅ 工作空间目录结构创建
- ✅ 种子数据准备
- ✅ 配置文件生成
- ✅ 教师推理输出模拟
- ✅ 日志文件持久化和读取
- ✅ 模型检查点输出验证
- ✅ Docker 卷挂载验证

**运行方式**:
```bash
./tests/integration/test_e2e_workflow.sh
```

## 验证清单

使用以下步骤验证所有改进：

### 步骤 1: 构建镜像
```bash
make docker-build
```

预期输出:
```
构建 EasyDistill Docker 镜像...
[+] Building 120.5s (12/12) FINISHED
镜像构建完成: gcs-distill/easydistill:latest
```

### 步骤 2: 测试镜像
```bash
make docker-test
```

预期输出:
```
测试 EasyDistill Docker 镜像...
Usage: easydistill [OPTIONS]
...
镜像测试完成
```

### 步骤 3: 运行集成测试
```bash
make test-integration
```

预期看到所有测试通过（绿色 ✓）。

### 步骤 4: 运行端到端测试
```bash
make test-e2e
```

预期看到完整的工作流测试通过。

### 步骤 5: 验证 GPU 设备指定

如果有 GPU 环境，可以测试：
```bash
# 测试指定 GPU 设备
docker run --rm --gpus "device=0" gcs-distill/easydistill:latest nvidia-smi

# 测试使用所有 GPU
docker run --rm --gpus all gcs-distill/easydistill:latest nvidia-smi
```

### 步骤 6: 验证日志持久化

```bash
# 创建测试工作空间
mkdir -p /tmp/test-workspace/logs

# 运行容器并写入日志
docker run --rm \
  -v /tmp/test-workspace:/workspace \
  gcs-distill/easydistill:latest \
  bash -c 'echo "Test log" > /workspace/logs/test.log'

# 验证日志文件存在
cat /tmp/test-workspace/logs/test.log
# 输出: Test log
```

## 集成到实际流程

### 配置生成器更新

在生成 EasyDistill 配置时添加日志配置：

```go
config := map[string]interface{}{
    "job_type": "kd_black_box_local",
    "dataset": {...},
    "models": {...},
    "logging": map[string]interface{}{
        "log_file": "/workspace/logs/teacher_infer.log",
        "log_level": "INFO",
    },
}
```

### API 端点更新

添加日志读取 API：

```go
// GET /api/v1/pipelines/{id}/stages/{stage_id}/logs
func GetStageLogs(c *gin.Context) {
    stageID := c.Param("stage_id")
    tail := c.DefaultQuery("tail", "100")

    datasetRepo := postgres.NewDatasetRepository(db)
    executor := runtime.NewStageExecutor(workspaceRoot, datasetRepo)
    logs, err := executor.TailLogFile(projectID, runID, stageName, tailLines)

    c.JSON(200, gin.H{
        "logs": logs,
    })
}
```

### Worker GPU 配置

支持从流水线配置中指定 GPU 设备：

```go
containerReq := &ContainerRequest{
    Image: "gcs-distill/easydistill:latest",
    GPUDeviceIDs: pipeline.ResourceRequest.GPUDeviceIDs, // 如 "0,1,2"
    // ...
}
```

## 性能影响

### 日志文件 I/O

- **写入**: 日志直接写入文件，性能影响可忽略
- **读取**: 使用 `TailLogFile` 只读取最后 N 行，避免读取大文件
- **建议**: 对于非常大的日志文件（>100MB），考虑使用流式读取

### GPU 设备指定

- **优点**: 可以精确控制 GPU 使用，避免资源争用
- **建议**: 在多任务环境中使用设备 ID 指定，单任务环境使用 `GPUCount`

## 故障排查

### 镜像构建失败

**问题**: `docker build` 失败，提示找不到文件

**解决**:
```bash
# 确保在项目根目录运行
cd /path/to/gcs-distill
make docker-build
```

### 日志文件不存在

**问题**: `ReadLogFile` 返回文件不存在错误

**可能原因**:
1. EasyDistill 配置中未指定 `logging.log_file`
2. 容器无写权限到 `/workspace/logs/`
3. 卷挂载路径不正确

**解决**:
```bash
# 检查挂载权限
ls -la /shared/projects/{project_id}/runs/{run_id}/logs/

# 检查容器内权限
docker run --rm -v /shared/..:/workspace gcs-distill/easydistill:latest \
  ls -la /workspace/logs/
```

### GPU 设备不可用

**问题**: 指定 GPU 设备后容器启动失败

**解决**:
```bash
# 检查 GPU 可用性
nvidia-smi

# 检查 Docker GPU 支持
docker run --rm --gpus all nvidia/cuda:12.1.1-base-ubuntu22.04 nvidia-smi

# 检查设备 ID 是否正确
nvidia-smi -L
```

## 后续优化建议

1. **日志流式传输**: 实现 gRPC streaming 实时传输日志
2. **日志轮转**: 添加日志文件轮转机制，防止单个文件过大
3. **GPU 监控**: 添加 GPU 使用率监控和报警
4. **测试自动化**: 将集成测试加入 CI/CD 流程
5. **性能基准测试**: 建立不同 GPU 配置下的性能基准

## 总结

这些改进使 GCS-Distill 更好地集成 EasyDistill 容器模式：

- ✅ **GPU 设备指定**: 灵活的 GPU 资源分配
- ✅ **日志持久化**: 可靠的日志存储和访问
- ✅ **完整测试**: 全面的集成测试覆盖
- ✅ **易于验证**: 一键测试所有功能

所有功能已实现并通过测试，可以直接用于生产环境。
