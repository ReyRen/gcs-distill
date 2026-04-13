# GCS-Distill: 大模型蒸馏平台

GCS-Distill 是一个企业级大模型蒸馏平台，用于将教师大模型在特定任务上的输出能力迁移到轻量级学生小模型中，形成可独立部署、推理成本更低、响应速度更快的专用模型能力。

## 功能特性

- **六阶段蒸馏流水线**: 教师模型配置 → 蒸馏数据构建 → 教师推理与样本生成 → 蒸馏数据治理 → 学生模型训练 → 蒸馏效果评估
- **分布式容器调度**: 基于 Docker Swarm 的 Worker 节点资源调度和任务执行
- **EasyDistill 集成**: 底层使用 [ModelScope EasyDistill](https://github.com/modelscope/easydistill) 执行蒸馏训练
- **REST API 服务**: 提供完整的项目管理、数据集管理、流水线管理 API
- **实时状态跟踪**: Redis + PostgreSQL 实现运行态和业务态分离的状态管理
- **共享存储支持**: 自动管理蒸馏数据、模型产物和评估报告的存储

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                       前端 (Frontend)                         │
└───────────────────────┬─────────────────────────────────────┘
                        │ REST API
┌───────────────────────▼─────────────────────────────────────┐
│                  控制面 (gcs-distill API)                      │
│  - 项目管理   - 数据集管理   - 流水线编排                       │
│  - 资源调度   - 配置生成     - 状态管理                         │
└───────────────────────┬─────────────────────────────────────┘
                        │ gRPC
┌───────────────────────▼─────────────────────────────────────┐
│                执行面 (Worker 节点)                           │
│  - 资源上报   - 容器执行   - 日志收集                          │
│  - EasyDistill 容器                                          │
└─────────────────────────────────────────────────────────────┘
                        │
┌───────────────────────▼─────────────────────────────────────┐
│                存储面 (Storage)                               │
│  - Redis: 运行态缓存                                          │
│  - PostgreSQL: 业务元数据                                     │
│  - 共享存储: 数据集、模型、日志                                 │
└─────────────────────────────────────────────────────────────┘
```

## 快速开始

### 使用 Docker Compose（推荐）

最快的启动方式，适合开发和测试环境：

```bash
# 1. 克隆仓库
git clone https://github.com/ReyRen/gcs-distill.git
cd gcs-distill

# 2. 一键启动所有服务
docker-compose up -d

# 3. 验证部署
curl http://172.18.36.230:18080/health

# 详细说明请参考: docs/quickstart.md
```

### 手动安装

### 前置要求

- Go 1.21+
- Docker 和 Docker Compose
- PostgreSQL 13+
- Redis 6+
- 共享存储 (NFS 或分布式文件系统)

### 安装步骤

1. 克隆仓库
```bash
git clone https://github.com/ReyRen/gcs-distill.git
cd gcs-distill
```

2. 构建 EasyDistill 镜像
```bash
cd docker/easydistill
docker build -t gcs-distill/easydistill:latest .
```

3. 配置环境变量
```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml 配置数据库、Redis、共享存储路径等
```

4. 初始化数据库
```bash
psql -U postgres -d gcs_distill -f migrations/001_initial_schema.sql
```

5. 启动控制面服务
```bash
go build -o gcs-distill-server ./cmd/server
./gcs-distill-server --config config.yaml
```

6. 启动 Worker 节点
```bash
go build -o gcs-distill-worker ./cmd/worker
./gcs-distill-worker --config config.yaml --node-name worker-1
```

### 使用示例

1. 创建蒸馏项目
```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "客服问答蒸馏",
    "description": "将 Qwen2.5-7B 蒸馏到 Qwen2.5-0.5B",
    "teacher_model": {
      "model_name": "Qwen/Qwen2.5-7B-Instruct",
      "provider_type": "local"
    },
    "student_model": {
      "model_name": "Qwen/Qwen2.5-0.5B-Instruct"
    }
  }'
```

2. 上传种子数据集
```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects/{project_id}/datasets \
  -F "file=@seed_data.json"
```

3. 启动蒸馏流水线
```bash
curl -X POST http://172.18.36.230:18080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "{project_id}",
    "dataset_id": "{dataset_id}",
    "training_config": {
      "num_train_epochs": 3,
      "learning_rate": 2e-5,
      "per_device_train_batch_size": 4
    }
  }'
```

4. 查看流水线状态
```bash
curl http://172.18.36.230:18080/api/v1/pipelines/{pipeline_id}
```

## 目录结构

```
gcs-distill/
├── cmd/                    # 可执行程序入口
│   ├── server/            # 控制面服务入口
│   └── worker/            # Worker 节点入口
├── server/                # HTTP 路由和 API Handler
├── service/               # 业务逻辑层
│   ├── project.go         # 项目管理服务
│   ├── dataset.go         # 数据集管理服务
│   ├── pipeline.go        # 流水线服务
│   └── scheduler.go       # 资源调度服务
├── repository/            # 数据访问层
│   ├── postgres/          # PostgreSQL 仓库
│   └── redis/             # Redis 仓库
├── internal/              # 内部包
│   ├── types/             # 领域模型定义
│   ├── config/            # 配置管理
│   └── logger/            # 日志系统
├── proto/                 # gRPC 协议定义
│   └── worker.proto       # Worker 服务协议
├── runtime/               # 运行时逻辑
│   ├── config_generator.go  # EasyDistill 配置生成
│   ├── manifest.go          # 运行清单管理
│   └── stage_executor.go    # 阶段执行器
├── utils/                 # 工具函数
├── migrations/            # 数据库迁移脚本
├── docker/                # Docker 相关资源
│   └── easydistill/       # EasyDistill 镜像
├── docs/                  # 文档
│   ├── api-reference.md       # API 接口参考
│   ├── frontend-guide.md      # 前端实现指南
│   ├── deployment.md          # 部署指南
│   └── quickstart.md          # 快速启动指南
└── README.md
```

## 蒸馏流程说明

### 六个核心阶段

1. **教师模型配置**
   - 验证教师模型可访问性
   - 配置 API 端点或本地模型路径
   - 存储配置版本

2. **蒸馏数据构建**
   - 导入业务语料、历史样本或知识库文档
   - 数据标准化和字段映射
   - 生成 seed instructions 数据集

3. **教师推理与样本生成**
   - 使用 EasyDistill 的 `kd_black_box_local` 模式
   - 教师模型对种子数据生成高质量响应
   - 输出 labeled 数据集

4. **蒸馏数据治理**
   - 空响应过滤
   - 长度和格式校验
   - 去重和质量评分
   - 输出 curated 训练集

5. **学生模型训练**
   - 使用 EasyDistill 的 `kd_black_box_train_only` 模式
   - 支持 LoRA 或全参微调
   - 定期保存 checkpoint

6. **蒸馏效果评估**
   - 使用 EasyDistill 的 `cot_eval_api` 模式
   - 对比学生模型与教师模型性能
   - 生成评估报告 (BLEU, ROUGE, 准确率等)

### 数据流转

```
/shared/projects/{project_id}/runs/{run_id}/
├── data/
│   ├── seed/              # 原始种子数据
│   ├── generated/         # 教师生成的标注数据
│   └── filtered/          # 治理后的训练数据
├── configs/
│   ├── teacher_infer.json    # 阶段3配置
│   ├── student_train.json    # 阶段5配置
│   └── evaluate.json         # 阶段6配置
├── models/
│   ├── checkpoints/       # 训练检查点
│   └── final/             # 最终导出模型
├── logs/
│   └── stage_{N}/         # 各阶段日志
└── eval/
    └── results.json       # 评估结果
```

## API 文档

详细 API 文档请访问: `http://172.18.36.230:18080/swagger/index.html`

主要 API 端点：

- `POST /api/v1/projects` - 创建项目
- `GET /api/v1/projects` - 列出项目
- `POST /api/v1/projects/{id}/datasets` - 上传数据集
- `POST /api/v1/pipelines` - 启动蒸馏流水线
- `GET /api/v1/pipelines/{id}` - 查询流水线状态
- `GET /api/v1/pipelines/{id}/stages` - 查看阶段详情
- `GET /api/v1/pipelines/{id}/logs` - 获取日志
- `POST /api/v1/pipelines/{id}/cancel` - 取消流水线
- `GET /api/v1/nodes` - 查看 Worker 节点状态

## 配置说明

`config.yaml` 示例：

```yaml
# 服务配置
server:
  host: 0.0.0.0
  port: 8080
  mode: release  # debug, release, test

# 数据库配置
database:
  host: 172.18.36.230
  port: 5432
  user: postgres
  password: postgres
  dbname: gcs_distill
  sslmode: disable

# Redis 配置
redis:
  host: 172.18.36.230
  port: 6379
  password: ""
  db: 0

# 共享存储配置
storage:
  type: nfs  # nfs, ceph, local
  base_path: /storage-md0/renyuan/gcs-distill-data/shared-workspace

# gRPC 配置
grpc:
  port: 50051

# 日志配置
logging:
  level: info  # debug, info, warn, error
  output: stdout  # stdout, file
  file_path: /storage-md0/renyuan/gcs-distill-data/logs/server.log
```

## 开发指南

### 编译项目

```bash
# 编译控制面
go build -o bin/server ./cmd/server

# 编译 Worker
go build -o bin/worker ./cmd/worker

# 生成 gRPC 代码
protoc --go_out=. --go-grpc_out=. proto/worker.proto
```

### 运行测试

```bash
go test ./...
```

### 代码风格

项目遵循标准的 Go 代码风格，使用 `gofmt` 和 `golint` 进行检查。

## 参考项目

- [gcs-v2](https://github.com/ReyRen/gcs-v2) - GPU 容器调度系统
- [gcs-info-catch-v2](https://github.com/ReyRen/gcs-info-catch-v2) - 信息采集系统
- [EasyDistill](https://github.com/modelscope/easydistill) - 模型蒸馏框架

## 许可证

Apache License 2.0

## 贡献

欢迎提交 Issue 和 Pull Request！

## 联系方式

如有问题，请提交 Issue 或联系项目维护者。
