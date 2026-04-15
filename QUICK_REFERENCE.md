# EasyDistill 集成快速参考

## 一键命令

```bash
# 构建镜像
make docker-build

# 测试镜像
make docker-test

# 运行所有测试
make test-all

# 运行集成测试
make test-integration

# 运行端到端测试
make test-e2e
```

## GPU 设备指定

```go
// 方式 1: 指定具体设备
containerReq := &ContainerRequest{
    GPUDeviceIDs: "0,1",  // 使用 GPU 0 和 1
}

// 方式 2: 指定数量
containerReq := &ContainerRequest{
    GPUCount: 2,  // 使用任意 2 个 GPU
}

// 方式 3: 使用所有
containerReq := &ContainerRequest{
    GPUCount: -1,  // 使用所有 GPU
}
```

## 日志文件读取

```go
// 创建执行器（需要传入数据集仓库）
datasetRepo := postgres.NewDatasetRepository(db)
executor := runtime.NewStageExecutor("/shared", datasetRepo)

// 读取完整日志
logs, _ := executor.ReadLogFile(projectID, runID, "teacher_infer")

// 读取最后 100 行
logs, _ := executor.TailLogFile(projectID, runID, "student_train", 100)
```

## 工作空间结构

```
/workspace/
├── configs/              # EasyDistill 配置文件
│   ├── teacher_infer.json
│   ├── student_train.json
│   └── evaluate.json
├── data/                 # 数据目录
│   ├── seed/            # 种子数据
│   ├── generated/       # 教师推理输出
│   └── filtered/        # 治理后数据
├── models/              # 模型输出
│   └── checkpoints/     # 训练检查点
└── logs/                # 日志文件（持久化）
    ├── teacher_infer.log
    ├── student_train.log
    └── evaluate.log
```

## 测试快速检查

```bash
# 检查镜像
docker images | grep easydistill

# 测试挂载
docker run --rm -v /tmp/test:/workspace gcs-distill/easydistill:latest ls /workspace

# 测试 GPU
docker run --rm --gpus all gcs-distill/easydistill:latest nvidia-smi

# 测试日志写入
docker run --rm -v /tmp/test:/workspace gcs-distill/easydistill:latest \
  bash -c 'echo "test" > /workspace/logs/test.log'
cat /tmp/test/logs/test.log
```

## 关键文件位置

| 文件 | 路径 |
|------|------|
| Dockerfile | `docker/easydistill/Dockerfile` |
| Proto 定义 | `proto/worker.proto` |
| Docker 客户端 | `internal/docker/client.go` |
| Worker 服务 | `cmd/worker/service.go` |
| 阶段执行器 | `runtime/stage_executor.go` |
| 集成测试 | `tests/integration/test_easydistill.sh` |
| 端到端测试 | `tests/integration/test_e2e_workflow.sh` |

## 文档位置

| 文档 | 路径 |
|------|------|
| 改进总结 | `IMPROVEMENTS_SUMMARY.md` |
| 验证指南 | `docs/VERIFICATION_GUIDE.md` |
| 技术细节 | `docs/easydistill-integration-improvements.md` |
| 测试文档 | `tests/integration/README.md` |

## 验证清单

- [ ] ✅ 镜像构建成功
- [ ] ✅ 镜像测试通过
- [ ] ✅ 集成测试通过
- [ ] ✅ 端到端测试通过
- [ ] ✅ GPU 功能验证（可选）
- [ ] ✅ 日志持久化验证
- [ ] ✅ Go 代码编译成功

## 故障排查

| 问题 | 解决方案 |
|------|----------|
| protoc 未安装 | `sudo apt-get install -y protobuf-compiler` |
| Docker 权限错误 | `sudo usermod -aG docker $USER` |
| 卷挂载失败 | 检查目录权限和 SELinux 设置 |
| GPU 不可用 | 安装 NVIDIA Docker Runtime |

## 下一步

1. 运行 `make docker-build` 构建镜像
2. 运行 `make test-all` 验证所有功能
3. 查看 `IMPROVEMENTS_SUMMARY.md` 了解详情
4. 参考 `docs/VERIFICATION_GUIDE.md` 进行完整验证
