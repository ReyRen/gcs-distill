# GCS-Distill

`gcs-distill` 是 GCS 系列里的蒸馏编排服务，底层蒸馏运行时基于 ModelScope EasyDistill。它负责项目、数据集、蒸馏流水线、阶段状态、EasyDistill 配置和共享存储清单；容器资源调度和实际执行统一交给 `gcs-v2` 与 `gcs-info-catch-v2`。

## 当前边界

- `gcs-distill`: 蒸馏控制面，维护业务对象、流水线阶段、运行目录和 EasyDistill 配置。
- `gcs-model-center-v2`: 模型资产与模型服务控制面，使用统一 MySQL 数据库配置风格。
- `gcs-v2`: GCS 统一调度面，负责节点选择、XPU 绑定、容器任务、运行态和终态历史。
- `gcs-info-catch-v2`: GCS 执行代理，负责 Docker 容器生命周期、设备绑定和容器日志。

`gcs-distill` 不直接操作 Docker、不维护 GPU 节点调度状态，也不内置独立数据库服务。阶段 3/5/6 的 EasyDistill 容器只通过 `POST /api/v1/tasks/container` 提交给 `gcs-v2`。

## 底层统一

数据库层已经按 `gcs-model-center-v2` 的方式统一：

- 使用 MySQL 与 `database/sql`，驱动为 `github.com/go-sql-driver/mysql`。
- 配置文件使用 TOML，`[database]` 字段与 model-center 保持一致。
- 默认数据库名为 `ai_market`，可以和 model-center 共用同一个库。
- distill 表统一使用 `distill_` 前缀，避免和 model-center 的 `mc_*` 表冲突。
- 服务启动时自动执行 MySQL DDL 迁移；`migrations/001_distill_mysql.sql` 仅作为离线初始化和审计脚本。

如果和 model-center 共用库，建议先确保数据库存在：

```sql
CREATE DATABASE IF NOT EXISTS ai_market CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

## 流水线阶段

1. `teacher_config`: 校验教师模型配置。
2. `dataset_build`: 创建运行目录，生成种子数据清单。
3. `teacher_infer`: 生成 `teacher_infer.json`，提交 EasyDistill 教师推理容器。
4. `data_govern`: 过滤、去重并拆分训练/测试数据。
5. `student_train`: 生成 `student_train.json`，提交 EasyDistill 学生训练容器。
6. `evaluate`: 生成 `evaluate.json`，提交 EasyDistill 评估容器并解析结果。

运行目录统一放在共享存储：

```text
{executor.workspace_root}/projects/{project_id}/runs/{pipeline_id}/
├── configs/
├── data/
│   ├── seed/
│   ├── generated/
│   └── filtered/
├── eval/
├── logs/
└── models/
    └── checkpoints/
```

这些路径会写入 EasyDistill 配置文件，并作为容器工作目录提交给 `gcs-v2`。共享存储必须在 `gcs-v2` 执行节点上以同一路径可访问。

## 快速启动

前置条件：

- Go 1.25+
- Docker / Docker Compose
- MySQL 8.0+，建议复用 model-center 的 `ai_market`
- 已运行并可访问的 `gcs-v2`
- 已接入 `gcs-v2` 的 `gcs-info-catch-v2`
- 多节点环境中的共享存储路径保持一致

本仓库的 Compose 只启动 distill server；MySQL、`gcs-v2` 和执行代理由对应项目独立部署。

```bash
cp config.toml config.local.toml
make docker-build
make docker-up
curl http://127.0.0.1:18080/health
```

本地直接运行：

```bash
make run-server
```

Swagger UI:

```text
http://127.0.0.1:18080/swagger/index.html
```

## 配置

关键配置在 `config.toml`：

```toml
[server]
host = "0.0.0.0"
port = 8080

[database]
enabled = true
driver = "mysql"
host = "127.0.0.1"
port = 3306
name = "ai_market"
user = "root"
password = ""
password_env = "AI_MARKET_DB_PASSWORD"
max_open_conns = 20
max_idle_conns = 5
conn_max_lifetime_seconds = 300

[storage]
base_path = "/mnt/shared/distill"
models_base_path = "/mnt/shared/distill/models"

[gcs]
base_url = "http://gcs-v2:8072/api/v1"
timeout_seconds = 30

[executor]
workspace_root = "/mnt/shared/distill"
max_concurrent = 5
runtime_image = "gcs-distill/easydistill:latest"
```

`database.password` 优先级高于 `database.password_env`。生产环境建议把密码放到 `AI_MARKET_DB_PASSWORD`，不要写入仓库配置。

## 资源选择

推荐使用 `resource_request.selected_resources` 表达完整节点和 XPU 选择：

```json
{
  "resource_request": {
    "gpu_count": 1,
    "selected_resources": [
      {
        "node_name": "gpu-node-01",
        "node_address": "172.18.36.225",
        "xpu_indices": [0]
      }
    ]
  }
}
```

`gpu_device_ids` 仍是请求模型字段的一部分，但只表达卡号，不包含节点信息；需要跨节点或精确绑定时应使用 `selected_resources`。

## 常用 API

- `GET /health`: 健康检查。
- `GET /swagger/openapi.json`: OpenAPI JSON。
- `POST /api/v1/projects`: 创建蒸馏项目。
- `GET /api/v1/projects`: 查询项目列表。
- `POST /api/v1/projects/{id}/datasets`: 上传项目数据集。
- `POST /api/v1/pipelines`: 创建并提交蒸馏流水线。
- `GET /api/v1/pipelines/{id}`: 查询流水线。
- `GET /api/v1/pipelines/{id}/stages`: 查询阶段列表。
- `GET /api/v1/pipelines/{id}/stages/{stage_id}/logs`: 查询阶段日志。
- `POST /api/v1/pipelines/{id}/cancel`: 取消流水线。
- `GET /api/v1/resources/nodes`: 代理查询 `gcs-v2` 节点快照。
- `GET /api/v1/resources/nodes/{name}`: 按名称查询 `gcs-v2` 节点快照。

## 构建与校验

```bash
make swagger
make build
make test
```

`make build` 和 Docker server 镜像构建都会先执行 `go run ./cmd/openapi`，确保嵌入的 Swagger/OpenAPI 文件在每次编译时自动更新并经过基础一致性检查。

## 目录结构

```text
gcs-distill/
├── cmd/
│   ├── openapi/        # OpenAPI 校验和格式化
│   └── server/         # HTTP 服务入口
├── docker/
│   ├── Dockerfile.server
│   └── easydistill/    # EasyDistill 运行镜像
├── docs/
├── internal/
│   ├── client/gcs/     # gcs-v2 HTTP 客户端
│   ├── config/
│   ├── logger/
│   └── types/
├── repository/mysql/   # 统一 MySQL 仓库实现
├── runtime/            # EasyDistill 配置、清单、阶段执行
├── server/             # Gin 路由、Handler、嵌入式 Swagger
├── service/            # 业务服务和流水线执行队列
└── migrations/
```

## 参考项目

- [gcs-v2](https://github.com/ReyRen/gcs-v2)
- [gcs-info-catch-v2](https://github.com/ReyRen/gcs-info-catch-v2)
- [gcs-model-center-v2](../gcs-model-center-v2)
- [EasyDistill](https://github.com/modelscope/easydistill)
