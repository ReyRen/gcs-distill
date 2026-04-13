# GCS-Distill 开发任务清单

## 当前状态：基础架构已完成 ✅

已完成的基础组件：
- ✅ 项目目录结构
- ✅ 领域模型定义
- ✅ gRPC 协议定义
- ✅ 数据库表设计
- ✅ 配置系统
- ✅ 日志系统
- ✅ 服务器和 Worker 入口
- ✅ EasyDistill Docker 配置
- ✅ 完整的文档（实施计划、README、开发指南）

## 下一步开发任务

### 优先级 P0：核心功能（必须）

#### 1. 数据访问层
- [ ] 实现 PostgreSQL 连接池 (`repository/postgres/db.go`)
- [ ] 实现 Redis 客户端 (`repository/redis/client.go`)
- [ ] 实现 ProjectRepository (`repository/postgres/project_repo.go`)
- [ ] 实现 DatasetRepository (`repository/postgres/dataset_repo.go`)
- [ ] 实现 PipelineRepository (`repository/postgres/pipeline_repo.go`)
- [ ] 实现 StageRepository (`repository/postgres/stage_repo.go`)
- [ ] 实现 NodeCache (`repository/redis/node_cache.go`)

#### 2. 业务逻辑层
- [ ] 实现 ProjectService (`service/project_service.go`)
  - CreateProject, GetProject, ListProjects, UpdateProject
- [ ] 实现 DatasetService (`service/dataset_service.go`)
  - UploadDataset, GetDataset, ProcessDataset
- [ ] 实现 PipelineService (`service/pipeline_service.go`)
  - CreatePipeline, GetPipeline, CancelPipeline
  - StartPipeline, AdvanceStage, UpdateStatus
- [ ] 实现 SchedulerService (`service/scheduler_service.go`)
  - FindAvailableNode, AllocateResources, ReleaseResources

#### 3. 运行时逻辑
- [ ] 实现 ConfigGenerator (`runtime/config_generator.go`)
  - GenerateTeacherInferConfig
  - GenerateStudentTrainConfig
  - GenerateEvaluationConfig
- [ ] 实现 StageExecutor (`runtime/stage_executor.go`)
  - ExecuteStage, MonitorStage, HandleStageResult
- [ ] 实现 ManifestManager (`runtime/manifest_manager.go`)
  - CreateManifest, SaveManifest, LoadManifest

#### 4. API 服务层
- [ ] 选择并集成 Web 框架 (Gin 或 Echo)
- [ ] 实现路由器 (`server/router.go`)
- [ ] 实现项目管理 Handler (`server/handlers/project_handler.go`)
- [ ] 实现数据集管理 Handler (`server/handlers/dataset_handler.go`)
- [ ] 实现流水线管理 Handler (`server/handlers/pipeline_handler.go`)
- [ ] 实现中间件 (`server/middleware/`)
  - CORS, 认证, 日志, 错误处理

#### 5. Worker 节点
- [ ] 实现 Docker 客户端封装 (`internal/docker/client.go`)
- [ ] 实现 gRPC Server (`cmd/worker/grpc_server.go`)
- [ ] 实现容器管理器 (`internal/docker/container_manager.go`)
- [ ] 实现资源检测 (`internal/resource/detector.go`)
- [ ] 实现心跳上报 (`cmd/worker/heartbeat.go`)

### 优先级 P1：重要功能

- [ ] 实现阶段重试机制
- [ ] 实现日志收集和聚合
- [ ] 实现数据治理脚本
- [ ] 添加单元测试（覆盖率 > 70%）
- [ ] 集成 Swagger 文档
- [ ] 添加 Prometheus 监控指标

### 优先级 P2：增强功能

- [ ] 实现本地教师模型支持
- [ ] 实现多节点分布式训练
- [ ] 实现 WebSocket 实时日志推送
- [ ] 实现评估报告可视化
- [ ] 添加前端管理界面
- [ ] 实现多租户隔离

## 快速开始开发

### 第一步：实现数据访问层
```bash
# 创建 PostgreSQL 连接
touch repository/postgres/db.go
touch repository/postgres/project_repo.go
touch repository/postgres/dataset_repo.go
touch repository/postgres/pipeline_repo.go

# 创建 Redis 客户端
touch repository/redis/client.go
touch repository/redis/node_cache.go
```

### 第二步：实现业务服务
```bash
# 创建服务层
touch service/project_service.go
touch service/dataset_service.go
touch service/pipeline_service.go
touch service/scheduler_service.go
```

### 第三步：实现 API
```bash
# 创建 HTTP 服务
touch server/router.go
touch server/handlers/project_handler.go
touch server/handlers/dataset_handler.go
touch server/handlers/pipeline_handler.go
touch server/middleware/cors.go
touch server/middleware/logger.go
```

### 第四步：集成到 main.go
在 `cmd/server/main.go` 中初始化所有组件并启动服务。

## 测试策略

1. **单元测试**: 每个函数/方法都需要测试
2. **集成测试**: 测试数据库、Redis 交互
3. **端到端测试**: 完整蒸馏流程测试
4. **性能测试**: 并发请求和资源调度测试

## 部署检查清单

- [ ] 数据库迁移脚本已执行
- [ ] Redis 已启动并可连接
- [ ] 共享存储已挂载
- [ ] Docker 已安装且权限正确
- [ ] EasyDistill 镜像已构建
- [ ] 配置文件已正确设置
- [ ] 日志目录已创建
- [ ] 防火墙规则已配置

## 参考资源

- 实施计划: `docs/implementation-plan.md`
- 开发指南: `docs/development-guide.md`
- 项目总结: `docs/project-summary.md`
- EasyDistill 文档: https://github.com/modelscope/easydistill

## 当前优先任务

**本周目标**: 完成数据访问层实现

1. 实现 PostgreSQL 连接池和基础 Repository
2. 实现 Redis 客户端和节点缓存
3. 编写数据访问层的单元测试
4. 更新 main.go 集成数据库连接

**下周目标**: 完成业务逻辑层

1. 实现 ProjectService 和 DatasetService
2. 实现 PipelineService 基础功能
3. 编写业务逻辑的单元测试
