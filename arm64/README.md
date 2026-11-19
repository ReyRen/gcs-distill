# GCS Distill 2 · ARM64 昇腾

控制端与 x86 使用同一套 EasyDistill 2 工作流、数据格式和 API。数据生成容器在 ARM CPU 上调用内网教师 API。学生训练容器加载用户目录中的基模，通过昇腾 NPU 运行 LLaMA-Factory。

| 用途 | 镜像 |
|---|---|
| 数据生成、扩充、评分和筛选 | `gcs/distill-data:2.0.0-aeb8ca3-arm64` |
| 学生 SFT / DPO 训练 | `gcs/distill-trainer:0.9.3-cann8-torch2.2-vl-arm64` |

训练镜像基于 CANN 8、PyTorch 2.2.0 与 TorchNPU 2.2.0。127.43 的已验证运行环境为 23.0.rc2 驱动和昇腾 910B。部署到其他硬件和驱动组合前，先执行下述镜像和算子检查。教师推理由管理员提供可达的 OpenAI 兼容服务，可通过模型中心选择登记服务，也可在任务中配置内网接口。

## 部署现有 ARM 平台

1. 确认控制端和数据、训练节点共享同一份 JFS，挂载路径一致。准备现有 MySQL 服务中的独立库和库级权限账号，服务启动时建表。127.43 使用 `gcs_distill_arm_v2_prod` 新库。
2. 在包目录运行 `sha256sum -c SHA256SUMS`。在数据、训练节点按需执行 `python3 scripts/load-images.py --bundle .`。只负责数据的节点可加 `--data-only`。
3. 在已确认卡空闲的训练节点执行 `python3 scripts/preflight.py --arch arm64 --shared-root /storage-root-jfs --npu-devices 2,5`。将卡号替换成该节点由 GCS 上报的物理号。
4. 使用包内 GCS 调度端和 Ascend Worker 更新对应服务。保留平台现有调度与工作节点配置。Worker 使用原生 ARM64 CGO 和 `-tags ascend`，需确认 `ldd` 能解析 `libdcmi.so`、`libascend_hal.so`、`libdrvdsmi_host.so`，并执行 Worker 部署目录的 `make check`。
5. 按 `config.example.toml` 配置模型蒸馏服务。将 `data_workers`、`training_workers` 设为对应镜像已加载并完成预检的节点，数据库密码通过 `GCS_DISTILL_DB_PASSWORD` 注入。用户的学生基模由 `scripts/import-model.py` 离线导入。
6. 启动后检查 `/health`、`/api/v1/runtime`、`/api/v1/resources/available`。前端以 `training_accelerator=ascend` 显示 NPU。已有接口的 `gpu_count` 字段承载选卡数量，`selected_resources[].xpu_indices` 填 GCS 返回的物理号，支持同一节点的单卡或多卡。

127.43 入口沿用 `http://172.18.127.43:8080`。当前配置的运行节点为 `smgpu23`，物理卡号为 `2`、`5`。

平台前端沿用 EasyDistill 2 的页面与接口。127.43 的代理前缀为 `/modelDistillIpAddress/api/v1`。从 `/runtime` 获取训练镜像和资源类型，或省略创建请求中的 `trainer_image` 以使用服务端默认值。

## 重建镜像

在 ARM64 Linux 构建机加载 `build-image.sh` 指定的两套基底。脚本核对基底镜像 ID 和上游提交，使用预下载的 ARM64 Python 3.10 wheelhouse 离线构建镜像。对接 x86 使用的同一 EasyDistill 与 LLaMA-Factory 源码提交。

```sh
git -C /path/to/easydistill archive --format=tar -o /tmp/distill-source.tar aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3
git -C /path/to/LLaMA-Factory archive --format=tar -o /tmp/trainer-source.tar ca75f1edf3cb50343ed1c98605141c3e22075b5f
bash arm64/build-image.sh data /tmp/distill-source.tar /path/to/data-wheels
bash arm64/build-image.sh trainer /tmp/trainer-source.tar /path/to/trainer-wheels
```

轮子版本见 `arm64/image/*-requirements.lock`。将两个镜像通过 `docker save` 导出，在 ARM 构建机通过 `offline/export-bundle.py --arch arm64` 生成交付包。训练权重和教师服务资源由平台现有资产目录和推理服务提供。
