# GCS-Distill 开发指南

## 快速开始

### 环境准备

1. **安装依赖工具**
```bash
# Go 1.21+
go version

# Protocol Buffers 编译器
brew install protobuf  # macOS
# 或
apt install protobuf-compiler  # Ubuntu

# Go gRPC 插件
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

2. **安装数据库**
```bash
# PostgreSQL
docker run --name postgres-gcs \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=gcs_distill \
  -p 5432:5432 \
  -d postgres:13

# Redis
docker run --name redis-gcs \
  -p 6379:6379 \
  -d redis:6
```

### 项目初始化

1. **克隆仓库**
```bash
git clone https://github.com/ReyRen/gcs-distill.git
cd gcs-distill
```

2. **安装 Go 依赖**
```bash
go mod tidy
```

3. **生成 gRPC 代码** (如需要)
```bash
make proto
```

4. **配置文件**
```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 修改数据库、Redis 等配置
```

5. **初始化数据库**
```bash
make db-init
# 或者手动执行
psql -U postgres -d gcs_distill -f migrations/001_initial_schema.sql
```

### 编译和运行

1. **编译项目**
```bash
# 编译服务端和 Worker
make build

# 仅编译服务端
make server

# 仅编译 Worker
make worker
```

2. **运行服务端**
```bash
# 使用 Makefile (开发模式)
make run-server

# 或者直接运行
./bin/gcs-distill-server --config config.yaml
```

3. **运行 Worker**
```bash
# 使用 Makefile (开发模式)
make run-worker

# 或者指定节点名称
./bin/gcs-distill-worker --config config.yaml --node-name worker-1
```

## 项目结构说明

```
gcs-distill/
├── cmd/                      # 可执行程序入口
│   ├── server/              # 控制面服务
│   │   └── main.go
│   └── worker/              # Worker 节点
│       └── main.go
├── server/                  # HTTP 路由层
│   ├── handlers/            # API Handler
│   ├── middleware/          # 中间件
│   └── router.go            # 路由配置
├── service/                 # 业务逻辑层
│   ├── project_service.go   # 项目管理服务
│   ├── dataset_service.go   # 数据集管理服务
│   ├── pipeline_service.go  # 流水线服务
│   └── scheduler_service.go # 资源调度服务
├── repository/              # 数据访问层
│   ├── postgres/            # PostgreSQL 仓库
│   │   ├── project_repo.go
│   │   ├── dataset_repo.go
│   │   └── pipeline_repo.go
│   └── redis/               # Redis 仓库
│       ├── node_cache.go
│       └── state_cache.go
├── internal/                # 内部包
│   ├── types/               # 领域模型
│   │   └── types.go
│   ├── config/              # 配置管理
│   │   └── config.go
│   └── logger/              # 日志系统
│       └── logger.go
├── proto/                   # gRPC 协议
│   ├── worker.proto
│   └── worker.pb.go         # 生成的代码
├── runtime/                 # 运行时逻辑
│   ├── config_generator.go  # EasyDistill 配置生成
│   ├── stage_executor.go    # 阶段执行器
│   └── manifest_manager.go  # 清单管理
├── utils/                   # 工具函数
│   ├── path.go              # 路径工具
│   └── validator.go         # 验证器
├── migrations/              # 数据库迁移
│   └── 001_initial_schema.sql
├── docker/                  # Docker 相关
│   └── easydistill/
│       ├── Dockerfile
│       └── README.md
├── docs/                    # 文档
│   ├── implementation-plan.md
│   └── development-guide.md
├── config.example.yaml      # 配置示例
├── Makefile                 # 构建脚本
└── README.md
```

## 开发流程

### 1. 添加新的 API 端点

1. 在 `internal/types/` 定义请求/响应结构
2. 在 `repository/` 添加数据访问方法
3. 在 `service/` 实现业务逻辑
4. 在 `server/handlers/` 实现 HTTP Handler
5. 在 `server/router.go` 注册路由
6. 编写测试

### 2. 添加新的阶段类型

1. 在 `internal/types/types.go` 定义阶段常量
2. 在 `runtime/config_generator.go` 实现配置生成
3. 在 `runtime/stage_executor.go` 实现执行逻辑
4. 在 `service/pipeline_service.go` 集成阶段

### 3. 修改数据库结构

1. 创建新的迁移文件 `migrations/00X_description.sql`
2. 更新 `internal/types/types.go` 中的模型
3. 更新相关的 repository 代码
4. 运行迁移测试

### 4. 扩展 gRPC 协议

1. 修改 `proto/worker.proto`
2. 运行 `make proto` 生成代码
3. 实现服务端逻辑 (Worker)
4. 实现客户端逻辑 (Server)

## 代码规范

### Go 代码风格

- 遵循 [Effective Go](https://golang.org/doc/effective_go.html)
- 使用 `gofmt` 格式化代码
- 使用 `golangci-lint` 进行静态检查

```bash
# 格式化代码
make fmt

# 代码检查
make lint
```

### 命名规范

- 包名: 小写单词，不使用下划线
- 文件名: 小写+下划线 (如 `project_service.go`)
- 函数/方法: 驼峰命名 (如 `CreateProject`)
- 常量: 大驼峰命名 (如 `StatusPending`)
- 接口: 以 `-er` 结尾 (如 `ProjectRepository`)

### 注释规范

- 公开的函数、类型、常量必须有文档注释
- 注释使用中文
- 复杂逻辑需要添加内联注释

```go
// CreateProject 创建蒸馏项目
// 参数:
//   project: 项目信息
// 返回:
//   创建的项目 ID 和可能的错误
func CreateProject(project *types.Project) (string, error) {
    // 验证项目名称
    if project.Name == "" {
        return "", errors.New("项目名称不能为空")
    }

    // 保存到数据库
    // ...
}
```

### 错误处理

- 使用 `errors.New()` 或 `fmt.Errorf()` 创建错误
- 错误信息使用中文，小写开头
- 包装错误时使用 `fmt.Errorf("上下文: %w", err)`

```go
if err != nil {
    return fmt.Errorf("创建项目失败: %w", err)
}
```

### 日志使用

```go
import "github.com/ReyRen/gcs-distill/internal/logger"

// 结构化日志
logger.Info("项目创建成功", zap.String("project_id", id))

// 格式化日志
logger.Infof("创建项目: %s", projectName)

// 错误日志
logger.Error("数据库连接失败", zap.Error(err))
```

## 测试

### 单元测试

```bash
# 运行所有测试
make test

# 运行指定包的测试
go test -v ./service/...

# 查看覆盖率
go test -cover ./...
```

### 集成测试

```bash
# 启动测试环境
docker-compose -f docker-compose.test.yaml up -d

# 运行集成测试
go test -tags=integration ./...
```

## 调试

### 本地调试

1. 使用 VS Code 或 GoLand 的调试功能
2. 配置 `.vscode/launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug Server",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/cmd/server",
            "args": ["--config", "config.yaml"]
        }
    ]
}
```

### 查看日志

```bash
# 服务端日志
tail -f /var/log/gcs-distill/server.log

# Docker 容器日志
docker logs -f <container-id>
```

## 常见问题

### 1. 数据库连接失败

```bash
# 检查 PostgreSQL 是否运行
docker ps | grep postgres

# 测试连接
psql -h 172.18.36.230 -U postgres -d gcs_distill
```

### 2. gRPC 代码生成失败

```bash
# 确保安装了 protoc 和插件
protoc --version
which protoc-gen-go
which protoc-gen-go-grpc

# 重新生成
make proto
```

### 3. 编译错误

```bash
# 清理并重新编译
make clean
go mod tidy
make build
```

### 4. Swagger 文档不更新

**问题描述:**
当你修改了 `server/apidocs/swagger/openapi.json` 或其他 Swagger 文档文件后，运行 `make docker-up` 重新编译，但访问 `/swagger` 端点时发现文档并没有更新。

**原因分析:**
Swagger 文档文件通过 Go 的 `embed.FS` 指令嵌入到二进制文件中（见 `server/apidocs/assets.go`）。当 Docker 构建时，Go 的构建缓存可能导致即使源文件已更新，嵌入的文件仍然是旧版本。

**解决方案:**

方法 1: 使用清理缓存的构建命令
```bash
# 清理 Docker 缓存并重新构建整个环境
make docker-up-clean

# 或只重建 server 服务
make docker-up-server-clean
```

方法 2: 手动清理 Docker 构建缓存
```bash
# 清理 Docker 构建缓存
make docker-builder-prune

# 然后正常启动
make docker-up
```

方法 3: 完全重新构建
```bash
# 停止所有服务
make docker-down

# 清理所有悬空镜像
make docker-prune

# 清理构建缓存
make docker-builder-prune

# 重新构建并启动
make docker-up-clean
```

**预防措施:**
Dockerfile 已经配置为在构建时清理 Go 构建缓存（`go clean -cache`），但如果问题仍然存在，请使用上述方法强制完全重新构建。

## 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m '添加某某功能'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 相关资源

- [EasyDistill 文档](https://github.com/modelscope/easydistill)
- [Go 官方文档](https://golang.org/doc/)
- [gRPC Go 快速开始](https://grpc.io/docs/languages/go/quickstart/)
- [PostgreSQL 文档](https://www.postgresql.org/docs/)
- [Redis 文档](https://redis.io/documentation)
