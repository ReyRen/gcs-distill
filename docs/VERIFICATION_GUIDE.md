# EasyDistill 容器集成验证指南

## 快速验证步骤

按照以下步骤验证所有改进功能：

### 前置要求

- Docker 已安装并运行
- Go 1.21+ （用于编译）
- protoc（如需重新生成 gRPC 代码）
- NVIDIA Docker Runtime（GPU 测试可选）

### 1. 生成 gRPC 代码（如果修改了 proto 文件）

```bash
# 安装 protoc（如果尚未安装）
# Ubuntu/Debian:
sudo apt-get install -y protobuf-compiler

# macOS:
brew install protobuf

# 生成代码
make proto
```

### 2. 构建 EasyDistill 镜像

```bash
make docker-build
```

**预期输出**:
```
构建 EasyDistill Docker 镜像...
[+] Building ...
镜像构建完成: gcs-distill/easydistill:latest
```

### 3. 测试镜像基本功能

```bash
make docker-test
```

**预期输出**:
```
测试 EasyDistill Docker 镜像...
Usage: easydistill [OPTIONS]
镜像测试完成
```

### 4. 运行容器集成测试

```bash
make test-integration
```

**预期看到**:
- ✓ 镜像存在
- ✓ 镜像可以正常运行
- ✓ 卷挂载正常
- ✓ 日志文件写入成功
- ✓ 模型输出目录可访问
- ✓ GPU 支持（如果可用）

### 5. 运行端到端工作流测试

```bash
make test-e2e
```

**预期看到**:
- ✓ 工作空间结构正确
- ✓ 种子数据已准备
- ✓ 配置文件已生成
- ✓ 日志文件可读取
- ✓ 模型检查点可访问

### 6. 编译项目

```bash
make build
```

**预期输出**:
```
编译服务端...
服务端编译完成: bin/gcs-distill-server
编译 Worker...
Worker 编译完成: bin/gcs-distill-worker
```

### 7. 验证 GPU 设备指定（可选）

如果有 GPU 环境，测试 GPU 设备指定功能：

```bash
# 测试使用特定 GPU
docker run --rm --gpus "device=0" gcs-distill/easydistill:latest nvidia-smi

# 测试使用多个 GPU
docker run --rm --gpus "device=0,1" gcs-distill/easydistill:latest nvidia-smi

# 测试使用所有 GPU
docker run --rm --gpus all gcs-distill/easydistill:latest nvidia-smi
```

### 8. 验证日志持久化

```bash
# 创建测试工作空间
TEST_WS=/tmp/test-logs
mkdir -p $TEST_WS/logs

# 运行容器写入日志
docker run --rm \
  -v $TEST_WS:/workspace \
  gcs-distill/easydistill:latest \
  bash -c 'echo "$(date): Test log entry" >> /workspace/logs/test.log'

# 验证日志存在
cat $TEST_WS/logs/test.log

# 清理
rm -rf $TEST_WS
```

## 验证新功能

### GPU 设备 ID 支持

**代码示例**:
```go
// 在 runtime/stage_executor.go 中使用
containerReq := &ContainerRequest{
    Image: "gcs-distill/easydistill:latest",
    Command: []string{"--config", "/workspace/configs/train.json"},
    GPUDeviceIDs: "0,1",  // 新增：指定使用 GPU 0 和 1
    // ...
}
```

### 日志文件读取

**代码示例**:
```go
// 创建执行器（需要传入数据集仓库）
datasetRepo := postgres.NewDatasetRepository(db)
executor := runtime.NewStageExecutor("/shared", datasetRepo)

// 读取完整日志
logs, err := executor.ReadLogFile(projectID, runID, "teacher_infer")
if err != nil {
    log.Fatal(err)
}
fmt.Println(logs)

// 读取最后 100 行
logs, err := executor.TailLogFile(projectID, runID, "student_train", 100)
if err != nil {
    log.Fatal(err)
}
fmt.Println(logs)
```

## 常见问题

### Q1: protoc 未安装

**错误**:
```
make: protoc: No such file or directory
```

**解决**:
```bash
# Ubuntu/Debian
sudo apt-get install -y protobuf-compiler protoc-gen-go protoc-gen-go-grpc

# macOS
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Q2: Docker 权限错误

**错误**:
```
permission denied while trying to connect to the Docker daemon socket
```

**解决**:
```bash
# 将用户添加到 docker 组
sudo usermod -aG docker $USER
newgrp docker

# 或使用 sudo
sudo make docker-build
```

### Q3: 集成测试失败

**错误**:
```
✗ 卷挂载失败
```

**解决**:
1. 检查 Docker 文件共享设置（macOS/Windows）
2. 确保测试目录有写权限
3. 检查 SELinux 设置（Linux）

### Q4: GPU 测试跳过

**信息**:
```
⚠ 未检测到 NVIDIA GPU，跳过 GPU 测试
```

**说明**: 这是正常的，如果没有 GPU 硬件，GPU 相关测试会自动跳过。

## 性能验证

### 日志读取性能

测试读取大型日志文件的性能：

```bash
# 创建大型测试日志（10MB）
for i in {1..100000}; do
    echo "$(date): Log line $i" >> /tmp/large.log
done

# 测试读取时间
time cat /tmp/large.log | tail -n 100

# 清理
rm /tmp/large.log
```

### GPU 分配验证

验证 GPU 正确分配：

```bash
# 启动容器并检查 GPU
docker run --rm --gpus "device=1" gcs-distill/easydistill:latest \
  nvidia-smi --query-gpu=index,name,memory.total --format=csv
```

## 集成到现有项目

### 1. 更新数据库 Schema（如需要）

如果要在数据库中存储 GPU 设备配置：

```sql
ALTER TABLE pipeline_runs
ADD COLUMN gpu_device_ids VARCHAR(50);
```

### 2. 更新 API

添加 GPU 设备 ID 到 API 请求：

```go
type CreatePipelineRequest struct {
    ProjectID      string `json:"project_id"`
    DatasetID      string `json:"dataset_id"`
    GPUDeviceIDs   string `json:"gpu_device_ids,omitempty"` // 新增
    // ...
}
```

### 3. 更新配置生成器

确保 EasyDistill 配置包含日志设置：

```go
config := map[string]interface{}{
    "job_type": "kd_black_box_local",
    "logging": map[string]interface{}{
        "log_file": "/workspace/logs/stage_name.log",
        "log_level": "INFO",
    },
    // ...
}
```

## 验证清单

完成以下所有项目即表示集成成功：

- [ ] ✅ 镜像构建成功
- [ ] ✅ 镜像基本测试通过
- [ ] ✅ 集成测试全部通过
- [ ] ✅ 端到端测试全部通过
- [ ] ✅ Go 代码编译成功
- [ ] ✅ GPU 设备可指定（如有 GPU）
- [ ] ✅ 日志文件可持久化
- [ ] ✅ 日志文件可读取
- [ ] ✅ 模型输出可访问

## 下一步

1. **生产部署**: 使用改进后的配置部署到生产环境
2. **监控**: 添加 GPU 使用率和日志大小监控
3. **优化**: 根据实际使用情况优化日志轮转和 GPU 分配策略
4. **文档**: 更新用户文档，说明新功能
5. **培训**: 向团队介绍新功能和使用方法

## 获取帮助

- 查看详细文档: `docs/easydistill-integration-improvements.md`
- 查看测试文档: `tests/integration/README.md`
- 提交问题: GitHub Issues
