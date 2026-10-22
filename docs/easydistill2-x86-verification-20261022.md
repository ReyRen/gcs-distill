# EasyDistill 2 x86 验收记录

日期：2026-10-22，Asia/Shanghai。

## 版本与部署

- 控制服务：`172.18.29.80`，`gcs-distill.service`，版本 `v2.0.1-x86-offline`。
- GCS manager：`172.18.29.80`，`gcs-V2.service`。
- GCS worker：`172.18.36.230`，节点 `specialai-3090-0`，`gcs-info-catch-V2.service`。
- 生产 distill 配置单独保存在 `/etc/gcs-distill/config.toml`，权限 0600；交付模板不含数据库密码。
- 官方 EasyDistill 源码固定为 `aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3`，15 份配方与官方文件逐字节核对。官方 LLaMA-Factory 固定为 v0.9.3 的 `ca75f1edf3cb50343ed1c98605141c3e22075b5f`。
- 数据镜像：`gcs/distill-data:2.0.0-aeb8ca3-x86_64`，ID `sha256:3ea860dcd708835a18b81d1a7d3d9f38e94f57e1ca6b3217a5aef0127a44b181`。
- 训练镜像：`gcs/distill-trainer:0.9.3-cu124-x86_64`，ID `sha256:7f92fac4d0c2576f3ed21301b6bbcc1408bcd4a619f36982629fc5200fc4e89f`；PyTorch 2.6.0 / CUDA 12.4。
- 二进制、镜像归档、依赖锁文件的最终校验值以离线包 `manifest.json` 和 `SHA256SUMS` 为准。worker 使用原生 Linux amd64、CGO/NVML，glibc 最低 2.31；没有使用 Ascend 构建标签。
- 三个仓库仅本地提交，未推送 GitHub。worker 发布增加 NVML 进程计数和采集时间，不包含工作区已有的推理模块修改。

## 实际通过的检查

三个仓库的 Go 测试通过；distill 的 go vet、OpenAPI JSON/schema 引用和 diff 检查通过。数据、训练镜像的依赖检查通过。Swagger 静态资源随程序内置，不再访问 CDN。

数据镜像内以 `--network none` 运行官方 CLI；平台端再经真实 manager/worker 执行以下链路：

| job_type | 输入记录 | 最终记录 | 结果 |
|---|---:|---:|---|
| instruct_distill | 2 | 2 | 通过 |
| cot_distill | 2 | 2 | 通过 |
| advanced_instruct_distill | 2 | 1 | 评分筛选后通过 |
| dpo_data_build | 2 | 2 | 优选/差选数据通过 |

平台验收覆盖配方及默认参数、配置预览脱敏、教师连通性、学生基模目录、项目/任务/数据集过滤、四阶段执行、产物预览/下载、数据资产登记及溯源、质量统计及直方图、任务克隆、取消、运行项目删除返回 409、训练配置交接、日志下载和跨 UID 拒绝。单元测试还覆盖损坏数据不发布、被引用数据集删除不丢文件、缺失镜像不拉取、取消状态不被旧写入覆盖及重启恢复。

学生训练分别通过以下实际验收，均使用共享存储内的 Qwen2.5-0.5B-Instruct：

| 入口 | GPU | SFT / DPO | 验证 |
|---|---|---|---|
| 原生训练镜像 | 7 | 两者均通过 | `--network none`、各 1 step、真实 LoRA 权重 |
| 平台完整任务 | 7 | 两者均通过 | 五阶段成功、真实权重 ZIP 下载 |
| 平台完整任务 | 6、7 | 两者均通过 | 双卡 DDP、五阶段成功、真实权重 ZIP 下载 |

平台单卡、双卡训练各生成约 17.6 MB 的 LoRA 适配器。容器只申请所选 GPU，distill NVIDIA 作业关闭 privileged；其他项目容器作业维持原行为。

服务端保留的验收报告：

- 80：`/root/gcs-distill2-build-20261022/control-api-final.json`。
- 80：`/root/gcs-distill2-build-20261022/student-gcs-gpu.json`、`student-gcs-ddp.json`。
- 230：`/storage-md0/renyuan/gcs-distill2-build-20261022/offline-student-smoke.log`。

### 固定页面接口与选卡更新验收

此次同步升级控制端、manager 和 specialai-3090-0 worker。`frontend_contract_smoke.py` 在真实 HTTP 服务通过：11 个页面配方及 SFT/CoT/DPO/Agent 参数预览、输出长度/并发覆盖、数据字段映射、项目设置、教师测试、学生目录、分数轮数、可编辑复用配置且原任务不变、跨项目/用户拒绝、pending 取消。资源验收验证真实 NVML 显存/进程数，6、7 号卡空闲可选；繁忙卡、失效卡号、多节点、重复/空卡号及卡数不匹配被拒绝，校验不预留 GPU。

更新后再跑四条真实 GCS 数据链路全部成功，双卡 SFT/DPO 均成功，下载的 adapter_model.safetensors 各为 17,640,136 字节。验证使用 Qwen2.5-0.5B-Instruct 和合成教师；GPU 0–5 原进程与其他 26 个容器保持运行。此次临时 fixture 已停止并清理。

最新三份报告位于 230 的 `/storage-md0/renyuan/offline/gcs-api-build-20261022/reports/`：`frontend-contract.json`、`control.json`、`student-ddp.json`，同时纳入离线包 `tests/reports/`。现有其他节点采集器未升级时显示监控未知、卡禁选，纳入蒸馏前按离线说明升级 worker、导入镜像和预检；不会伪造其空闲状态。

## 清理与影响范围

已删除服务器上属于旧蒸馏的容器、三个 v1 镜像和前一次临时 v2 数据镜像。清理按明确镜像/容器归属执行，不使用全局 prune，不删除用户共享数据，也不改动其他项目的镜像。

按用户明确授权，停止 GPU 6、7 原有的 `dynamicrag-bge-m3` 和 `dynamicrag-bge-reranker-v2-m3` 容器；容器和镜像保留，可执行 `docker start dynamicrag-bge-m3 dynamicrag-bge-reranker-v2-m3` 恢复。GPU 0–5 的任务未停止。ARM 主机未部署此次变更。

## 验收边界

教师使用带认证的确定性合成 API，用于核对执行、状态和文件合同，不证明真实教师质量。学生验收验证 LoRA 训练和导出功能，尚未评估模型效果；全量微调、真实 Agent、视觉/视频模型效果没有做 GPU 实测。15 份官方配方的配置接入通过，不等同于上述模型效果通过。

前端 mock 是交互示意，正式实现须按最新 `easydistill2-frontend-handoff.md` 和 OpenAPI 接入。平台基础环境要求见离线说明：共享挂载、Docker/网络、GPU 驱动及 Toolkit、内网教师服务、完整学生基模和认证 UID 网关需要由平台提前准备。
