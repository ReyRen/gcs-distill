# GCS 统一执行链路

本文说明 `gcs-distill` 接入 GCS 系列统一底座后的职责边界、数据库边界和执行链路。

## 服务职责

`gcs-distill` 只做蒸馏业务编排：

- 管理项目、数据集、流水线和阶段元数据。
- 生成 EasyDistill 的阶段配置文件。
- 写入种子数据、教师生成数据、治理后的训练/测试数据和评估结果。
- 维护阶段状态、日志路径和输出清单。

`gcs-model-center-v2` 负责模型资产、模型服务和模型中心业务对象。`gcs-distill` 的数据库配置方式与它保持一致，默认连接同一个 MySQL 数据库 `ai_market`，但使用 `distill_` 表前缀隔离表结构。

`gcs-v2` 负责统一资源调度：

- 查询和选择节点资源。
- 处理 `selected_resources` 手动资源选择。
- 软占用和释放 XPU。
- 创建容器任务、下发执行请求、保存运行态与终态历史。

`gcs-info-catch-v2` 负责执行面：

- 创建、监控和删除 Docker 容器。
- 绑定 NVIDIA GPU 或 Ascend NPU。
- 将容器日志写入共享存储。
- 同步容器退出状态。

## 数据库边界

`gcs-distill` 使用 MySQL + `database/sql`，配置文件为 `config.toml`：

```toml
[database]
enabled = true
driver = "mysql"
host = "172.18.127.67"
port = 3306
name = "ai_market"
user = "root"
password = "!Market4AI"
password_env = ""
max_open_conns = 20
max_idle_conns = 5
conn_max_lifetime_seconds = 300
```

数据库设计约定：

- model-center 表保持 `mc_*` 前缀。
- distill 表使用 `distill_*` 前缀。
- 服务启动时会连接配置中的 MySQL，并执行 `repository/mysql/schema.go` 中的 `CREATE TABLE IF NOT EXISTS distill_*`。
- 这里不是业务数据迁移，也不是从数据库获取表结构；它只是幂等地确认 distill 需要的表存在。已有表不会被重建。
- 不再保留独立离线迁移脚本，避免 `schema.go` 与 SQL 文件两套表结构漂移。

## 阶段执行

阶段 1、2、4 在 `gcs-distill` 进程内执行，因为它们只涉及配置校验、目录准备和数据治理。

阶段 3、5、6 通过 `POST /api/v1/tasks/container` 提交到 `gcs-v2`：

| 阶段 | EasyDistill 配置 | 容器任务 |
| --- | --- | --- |
| `teacher_infer` | `configs/teacher_infer.json` | 教师推理与样本生成 |
| `student_train` | `configs/student_train.json` | 学生模型训练 |
| `evaluate` | `configs/evaluate.json` | 蒸馏效果评估 |

容器请求只携带蒸馏运行时真正需要的信息：容器名、镜像、共享工作目录、配置路径、日志路径、环境变量、XPU 数量和可选的手动资源选择。`gcs-distill` 不构建 runtime 镜像，也不会部署到 worker 节点；Dockerfile 应放在 EasyDistill 或专门的镜像发布仓库。`executor.runtime_image` 必须指向已经推送、且 worker 节点可拉取的镜像。

为了避免官方 `kd_black_box_local` 入口在教师推理阶段自动串联训练，`gcs-distill` 会显式设置容器 `command` 和 `args`：

| 阶段 | command | args |
| --- | --- | --- |
| `teacher_infer` | `python` | `-m easydistill.kd.infer --config {config_path}` |
| `student_train` | `accelerate` | `launch [--multi_gpu] --num_processes {xpu_count} --module easydistill.kd.train --config {config_path}` |
| `evaluate` | `python` | `-m easydistill.eval.data_eval --config {config_path}` |

因此 runtime 镜像不需要把默认 ENTRYPOINT 设成 `easydistill`，但必须能在 Python 环境中导入 `easydistill`，并安装 `accelerate`。

## 共享存储约定

运行目录示例：

```text
/storage-root-jfs/distill/projects/{project_id}/runs/{pipeline_id}
```

`gcs-info-catch-v2` 需要把这个目录以同一路径挂载进容器。EasyDistill 配置中的所有输入、输出和日志路径都使用共享存储绝对路径，因此不再依赖 `gcs-distill` 的私有执行入口或私有工作目录约定。

## 资源选择

推荐使用 `resource_request.selected_resources`：

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

`gpu_device_ids` 只表达卡号，无法表达节点，因此不适合作为 GCS 系列跨节点手动资源选择的主字段。

## 清理后的仓库边界

`gcs-distill` 仓库只保留 HTTP 控制面、MySQL 业务数据和蒸馏编排逻辑。所有容器执行都经过：

```text
gcs-distill -> gcs-v2 -> gcs-info-catch-v2 -> Docker / GPU / NPU
```

训练、推理、模型中心和蒸馏项目共享同一套资源入口和数据库配置风格，避免多个服务各自维护节点占用状态或数据库连接方式。
