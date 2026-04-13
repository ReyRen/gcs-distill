# gcs-distill 实施计划

## 1. 项目目标

gcs-distill 用于把教师大模型在特定任务上的输出能力迁移到轻量级学生小模型中，形成可独立部署、推理成本更低、响应速度更快的专用模型能力。

系统需要围绕以下六个环节完成统一编排：

1. 教师模型配置
2. 蒸馏数据构建
3. 教师推理与样本生成
4. 蒸馏数据治理
5. 学生模型训练
6. 蒸馏效果评估

运行方式参考 gcs-v2 和 gcs-info-catch-v2：

- 前端通过 REST API 与 gcs-distill 交互
- gcs-distill 负责任务建模、状态管理、资源调度和阶段编排
- gcs-distill 通过 gRPC 调度 worker 节点执行任务容器
- worker 节点拉起 EasyDistill 镜像并执行 `easydistill --config <config>` 或底层脚本命令
- 共享存储用于保存数据、日志、配置、模型和评估结果

## 2. 总体架构

建议采用三层架构：

- 控制面：gcs-distill API 服务
- 执行面：worker 节点上的通用容器执行代理
- 存储面：Redis + 关系型数据库 + 共享存储

职责划分如下：

### 2.1 控制面职责

- 提供项目、数据集、蒸馏任务、运行日志、评估报告等 REST API
- 管理任务状态机和阶段编排
- 管理节点资源视图和调度决策
- 生成 EasyDistill 配置文件和运行参数
- 汇总阶段结果并推送给前端

### 2.2 执行面职责

- 定期上报节点资源状态
- 接收 gcs-distill 的 gRPC 调度请求
- 拉取镜像、创建容器、挂载目录、注入环境变量、执行命令
- 回写容器状态、退出码和日志信息

### 2.3 存储面职责

- Redis：保存运行态资源池、任务状态、阶段状态
- MySQL 或 PostgreSQL：保存业务元数据、任务定义、评估报告索引
- 共享存储：保存原始数据、清洗数据、配置、日志、checkpoint、导出模型

## 3. 核心对象设计

建议不要只保留单一 Task 对象，而是拆成四层对象：

### 3.1 DistillProject

表示一个蒸馏项目，包含以下信息：

- 项目名称
- 业务场景
- 教师模型配置版本
- 学生模型配置版本
- 评测配置版本
- 数据集集合

### 3.2 PipelineRun

表示一次完整蒸馏执行，包含以下信息：

- run_id
- project_id
- 当前阶段
- 总体状态
- 资源申请信息
- 触发方式
- 时间戳

### 3.3 StageRun

表示一次阶段运行，对应六个固定阶段之一：

- teacher_config
- dataset_build
- teacher_infer
- data_govern
- student_train
- evaluate

每个阶段需要记录：

- stage_id
- stage_type
- stage_status
- input_manifest
- output_manifest
- log_path
- metrics
- retry_count

### 3.4 ContainerRun

表示某个 worker 上的具体容器实例，至少记录：

- container_name
- image
- node_name
- node_addr
- command
- args
- envs
- mounts
- xpu_allocation
- exit_code
- started_at
- finished_at

## 4. 六个环节的实现方式

### 4.1 教师模型配置

实现方式：

- 建立统一 TeacherProvider 抽象
- 支持 `api` 和 `local` 两种模式
- 教师配置版本化管理
- 敏感信息通过 secret 引用，不直接明文写库

建议字段：

- provider_type
- model_name
- endpoint
- api_secret_ref
- temperature
- max_tokens
- concurrency
- timeout_seconds

### 4.2 蒸馏数据构建

实现方式：

- 提供多来源导入能力：业务语料、历史样本、知识库文档
- 做统一字段映射和样本标准化
- 支持 prompt 模板渲染
- 输出标准中间数据集

建议处理步骤：

1. 数据导入
2. 字段映射
3. 文本切片
4. 标签/元数据补全
5. 训练集、验证集、测试集拆分

### 4.3 教师推理与样本生成

实现方式：

- 为每次运行生成独立 EasyDistill 配置文件
- 由 worker 拉起 easydistill 镜像执行推理阶段
- 统一采集推理耗时、token 消耗、失败样本

首版建议优先支持：

- API 型教师模型
- 单 worker 的批量样本生成

### 4.4 蒸馏数据治理

实现方式：

- 独立阶段运行，不与训练脚本耦合
- 规则过滤与模型评估过滤结合
- 支持人工抽样复核

建议治理规则：

- 空响应过滤
- 超长/异常文本过滤
- 重复样本去重
- 敏感信息脱敏
- 质量评分筛选
- 标签一致性校验

### 4.5 学生模型训练

实现方式：

- 使用治理后的数据集作为训练输入
- 使用 EasyDistill 训练命令执行训练
- 将训练参数和产物版本化

首版范围建议：

- 单 worker 多卡
- LoRA 或全参微调二选一，优先 LoRA
- 支持训练日志、checkpoint、最佳模型导出

暂不建议首版直接做多节点分布式训练，原因是：

- worker 侧协议还不够通用
- 容器间 rank 和 rendezvous 编排复杂
- 容错和重试代价高

### 4.6 蒸馏效果评估

实现方式：

- 独立评测阶段，不把训练完成等同于任务成功
- 对学生模型、教师模型和基线模型做统一对比
- 输出结构化评测报告

建议指标：

- 任务效果指标：EM、F1、ROUGE、BLEU、BERTScore
- 模型对比指标：teacher vs student 胜率
- 成本指标：推理时延、吞吐、token 成本、显存占用
- 线上部署指标：模型大小、冷启动、平均响应时间

## 5. 服务分层建议

建议整体代码结构参考 gcs-v2 的分层方式：

- `server/`：HTTP 路由、OpenAPI、Swagger
- `service/`：项目服务、数据集服务、流水线服务、训练服务、评估服务
- `repository/`：Redis 和数据库访问封装
- `internal/types/`：统一领域模型和状态定义
- `proto/`：gRPC 协议
- `runtime/`：容器命令拼装、manifest 生成、配置文件渲染
- `utils/`：配置、日志、存储、公共工具

## 6. 调度与运行方式

### 6.1 调度原则

不要把整个蒸馏任务一次性占满资源，应该按阶段调度：

- `dataset_build`：CPU 优先
- `teacher_infer(api)`：CPU 优先
- `teacher_infer(local)`：GPU/NPU 资源
- `data_govern`：CPU 优先
- `student_train`：GPU/NPU 资源
- `evaluate`：CPU 或轻量 GPU

### 6.2 调度策略

建议复用 gcs-v2 的资源池和调度思路：

- pack：减少资源碎片
- spread：均匀分散
- balanced：平衡使用率

### 6.3 任务状态机

建议统一状态：

- pending
- scheduled
- preparing
- running
- succeeded
- failed
- canceled

阶段状态和总任务状态分别维护。

## 7. gRPC 协议扩展建议

当前参考项目里的 worker gRPC 更适合特定训练任务，gcs-distill 需要更通用的容器执行协议。

建议扩展或重构为以下接口：

- `PullImage`
- `RunContainer`
- `StopContainer`
- `DeleteContainer`
- `InspectContainer`
- `ReadContainerLogs`
- `MarkResourceOccupied`

`RunContainer` 至少需要支持这些参数：

- image
- container_name
- command
- args
- envs
- mounts
- workdir
- shm_size
- network
- resource_request
- log_path
- artifact_path

返回值至少包含：

- container_id
- container_ip
- exit_code
- error_message

## 8. 数据与目录规划

建议共享存储按项目和运行实例组织：

```text
distill/
  projects/<project_id>/
    runs/<run_id>/
      sources/
      dataset/raw/
      dataset/generated/
      dataset/curated/
      configs/
      logs/
      checkpoints/
      eval/
      manifests/
```

目录说明：

- `sources/`：原始导入数据
- `dataset/raw/`：标准化前数据
- `dataset/generated/`：教师生成后的样本
- `dataset/curated/`：清洗筛选后的训练数据
- `configs/`：各阶段下发到容器的配置
- `logs/`：各阶段日志
- `checkpoints/`：训练中间产物和导出模型
- `eval/`：评测结果
- `manifests/`：运行快照和元数据归档

## 9. REST API 建议

建议首版提供以下 API 组：

### 9.1 项目管理

- `POST /api/v1/projects`
- `GET /api/v1/projects`
- `GET /api/v1/projects/{id}`
- `PUT /api/v1/projects/{id}`

### 9.2 数据集管理

- `POST /api/v1/datasets/import`
- `GET /api/v1/datasets/{id}`
- `POST /api/v1/datasets/{id}/preview`
- `POST /api/v1/datasets/{id}/govern`

### 9.3 蒸馏运行管理

- `POST /api/v1/pipelines`
- `GET /api/v1/pipelines`
- `GET /api/v1/pipelines/{id}`
- `POST /api/v1/pipelines/{id}/cancel`
- `POST /api/v1/pipelines/{id}/retry-stage`

### 9.4 日志和产物

- `GET /api/v1/pipelines/{id}/logs`
- `GET /api/v1/pipelines/{id}/artifacts`
- `GET /api/v1/pipelines/{id}/evaluation`

### 9.5 资源观测

- `GET /api/v1/health`
- `GET /api/v1/brain`
- `GET /api/v1/nodes`

## 10. 数据库表建议

建议至少包含以下表：

- `distill_projects`
- `distill_project_versions`
- `datasets`
- `dataset_versions`
- `pipeline_runs`
- `stage_runs`
- `container_runs`
- `evaluation_reports`
- `artifacts`

其中 Redis 保存：

- 实时资源池
- 节点快照
- 运行态任务状态
- 容器短周期状态缓存

## 11. 首版实现范围

首版 MVP 建议控制范围，只做一条稳定闭环：

1. 项目创建
2. 数据导入与标准化
3. API 型教师样本生成
4. 数据治理
5. 单 worker 多卡学生训练
6. 基础评估与报告输出

首版不做：

- 多节点分布式训练
- 多租户隔离
- 复杂工作流可视化编排器
- 在线模型发布

## 12. 推荐开发里程碑

### 里程碑 1：基础骨架

- 初始化 Go 项目结构
- 配置系统、日志系统、Redis、数据库连接
- 接入 REST API 基础框架
- 建立基础领域模型和状态机

### 里程碑 2：资源与执行链路

- 复用或改造 worker gRPC 协议
- 完成节点资源同步
- 完成通用容器拉起、删除、日志读取

### 里程碑 3：蒸馏流水线 MVP

- 项目管理 API
- 数据集导入与构建
- 教师推理阶段
- 数据治理阶段
- 学生训练阶段
- 基础评估阶段

### 里程碑 4：产品化补齐

- Swagger / OpenAPI
- 阶段重试
- 失败恢复
- 运行报告和产物索引
- 更细粒度的指标采集

### 里程碑 5：增强能力

- 本地教师模型推理
- 模板化任务配置
- 多阶段评测
- 多节点分布式训练

## 13. 当前建议的落地顺序

推荐按下面顺序推进：

1. 先定义领域模型、数据库模型和任务状态机
2. 再定义 gRPC 协议和 worker 执行能力
3. 再搭 API 服务和基础资源调度
4. 然后打通 EasyDistill 容器执行链路
5. 最后补评测、重试和前端展示接口

## 14. 关键设计原则

- EasyDistill 是执行引擎，不是整个平台
- 任务必须按阶段编排，不能只做一个长任务
- 运行态和业务态分离，Redis 与数据库职责分开
- 数据治理必须独立成阶段
- 首版优先单 worker 多卡，避免直接进入多节点复杂度
- 每次运行必须生成可复现的 manifest 和配置快照

## 15. EasyDistill 核心命令理解

基于对 EasyDistill 源码的分析，其核心功能和命令映射如下：

### 15.1 命令行入口

EasyDistill 通过统一的 CLI 入口执行：
```bash
easydistill --config <config-file-path>
```

配置文件中的 `job_type` 字段决定执行哪种类型的蒸馏任务。

### 15.2 支持的任务类型

#### 知识蒸馏训练
- `kd_black_box_train_only`: 黑盒蒸馏训练（仅训练阶段）
- `kd_white_box_train_only`: 白盒蒸馏训练（仅训练阶段）
- `kd_black_box_local`: 黑盒蒸馏完整流程（推理+训练）
- `kd_white_box_local`: 白盒蒸馏完整流程（推理+训练）

#### 排序优化
- `rank_dpo`: DPO (Direct Preference Optimization) 训练
- `rank_orpo`: ORPO 排序优化训练

#### 强化学习
- `rl_ppo`: PPO (Proximal Policy Optimization) 训练
- `rl_grpo`: GRPO 训练
- `rl_reward_train`: 奖励模型训练
- `rl_reward_infer`: 奖励模型推理

#### 评估
- `cot_eval_api`: CoT (Chain-of-Thought) 评估
- `mmcot_eval_api`: 多模态 CoT 评估

#### AgentKD 数据生成
- `agentkd_data_gen`: 生成工具使用任务和轨迹数据

### 15.3 配置文件结构

EasyDistill 的配置文件为 JSON 格式，主要包含以下部分：

```json
{
    "job_type": "kd_black_box_local",
    "dataset": {
        "instruction_path": "输入数据路径",
        "labeled_path": "标注数据路径",
        "template": "chat_template/chat_template_kd.jinja",
        "seed": 42
    },
    "inference": {
        "enable_chunked_prefill": true,
        "temperature": 0.8,
        "max_model_len": 4096,
        "max_new_tokens": 512
    },
    "models": {
        "teacher": "教师模型路径",
        "student": "学生模型路径"
    },
    "training": {
        "output_dir": "./result/",
        "num_train_epochs": 3,
        "per_device_train_batch_size": 1,
        "learning_rate": 2e-5
    }
}
```

### 15.4 gcs-distill 六阶段与 EasyDistill 的映射

基于 EasyDistill 的实际能力，将 gcs-distill 六阶段映射如下：

| gcs-distill 阶段 | EasyDistill 任务类型 | 说明 |
|-----------------|---------------------|------|
| 1. 教师模型配置 | 无需容器 | 仅配置验证和存储 |
| 2. 蒸馏数据构建 | 自定义 Python 脚本 | 数据预处理和标准化 |
| 3. 教师推理与样本生成 | kd_black_box_local (推理阶段) | 教师模型生成训练样本 |
| 4. 蒸馏数据治理 | 自定义过滤脚本 | 数据清洗和质量筛选 |
| 5. 学生模型训练 | kd_black_box_train_only | 学生模型微调训练 |
| 6. 蒸馏效果评估 | cot_eval_api | 模型效果评测 |

### 15.5 容器执行策略

**阶段 3: 教师推理**
```bash
docker run --rm \
  -v /shared/projects/{project_id}/runs/{run_id}:/workspace \
  --gpus all \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/teacher_infer.json
```

配置文件示例：
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
        "teacher": "Qwen/Qwen2.5-7B-Instruct"
    }
}
```

**阶段 5: 学生训练**
```bash
docker run --rm \
  -v /shared/projects/{project_id}/runs/{run_id}:/workspace \
  --gpus all \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/student_train.json
```

配置文件示例：
```json
{
    "job_type": "kd_black_box_train_only",
    "dataset": {
        "instruction_path": "/workspace/data/filtered/train.json",
        "template": "chat_template/chat_template_kd.jinja"
    },
    "models": {
        "teacher": "Qwen/Qwen2.5-7B-Instruct",
        "student": "Qwen/Qwen2.5-0.5B-Instruct"
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

**阶段 6: 效果评估**
```bash
docker run --rm \
  -v /shared/projects/{project_id}/runs/{run_id}:/workspace \
  --gpus device=0 \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/evaluate.json
```

### 15.6 关键实现要点

1. **配置生成器**: gcs-distill 控制面需要根据用户输入和阶段参数动态生成 EasyDistill 配置文件
2. **路径映射**: 容器内 `/workspace` 目录必须映射到共享存储的运行实例目录
3. **资源分配**: GPU 分配通过 Docker `--gpus` 参数控制，需要在 gRPC 调度请求中明确指定
4. **日志收集**: 容器标准输出需要实时收集并存储到 `/workspace/logs/stage_N/` 目录
5. **状态监控**: 通过容器退出码判断阶段执行成功与否，非零退出码表示失败
6. **产物管理**: 训练产物（checkpoint、模型）自动保存到配置指定的 output_dir，评估结果需要解析并存入数据库

### 15.7 数据流转示意

```
用户上传种子数据
  ↓
阶段2: 数据标准化脚本处理
  ↓ /workspace/data/seed/instructions.json
阶段3: EasyDistill 教师推理 (kd_black_box_local)
  ↓ /workspace/data/generated/labeled.json
阶段4: 数据治理脚本过滤
  ↓ /workspace/data/filtered/train.json
阶段5: EasyDistill 学生训练 (kd_black_box_train_only)
  ↓ /workspace/models/checkpoints/
阶段6: EasyDistill 评估 (cot_eval_api)
  ↓ /workspace/eval/results.json
```

### 15.8 首版实现建议调整

基于对 EasyDistill 的深入理解，首版实现应：

1. **优先支持 API 型教师模型**: 避免在 worker 上加载大模型，降低资源需求
2. **简化数据治理**: 实现基础的规则过滤（空响应、长度限制、去重），高级质量评分后续迭代
3. **使用 EasyDistill 原生配置**: 不做额外封装，直接生成标准 JSON 配置文件
4. **复用 EasyDistill 的模板系统**: 使用其自带的 Jinja 模板而非自定义格式
5. **监控容器执行**: 重点关注 EasyDistill 的日志输出，识别训练进度和错误信息