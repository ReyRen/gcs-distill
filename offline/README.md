# x86 离线交付

客户现场无需 Go、pip、Git 或外网。交付包包括三个 Linux amd64 二进制、数据和训练两份完整 Docker 镜像、配置模板、校验清单及安装/预检脚本。运行时不会补下载依赖。

## 平台提前准备

1. Linux x86_64、现有 Docker 和 worker 的容器网络，须能访问内网教师服务；使用 Swarm overlay 时需允许普通容器附加。沿用平台实际网络，不要求新建名为 myNet 的网络。GPU 节点需 NVIDIA 驱动及 Container Toolkit，支持镜像 CUDA 12.4。Docker、驱动、Toolkit、存储客户端属于平台基础环境，平台需按客户操作系统另外准备其离线安装包。
2. 控制服务与所有执行节点以相同绝对路径挂载真正共享的存储，允许读写用户目录。平台/gcs-s3 创建用户根目录；distill 按需创建子目录。候选文件放 `datasets/candidates`。
3. 学生基模完整入库：config、tokenizer 配置、词表及全部权重分片；视觉模型还需要 processor、chat template 等原始配套文件。禁止提交 Hugging Face 仓库名等待下载。LoRA 输出是适配器，使用时需要原始基模；全量训练输出完整权重。
4. MySQL、Redis、GCS manager/worker、模型中心正常。教师须为 worker 可访问的内网 OpenAI 兼容服务；离线允许访问内网。
5. 平台认证网关注入 UID。数据库密码放受保护配置或 EnvironmentFile，不进入交付模板。
6. 规划用户配额、镜像和检查点空间。挂载、网络、基础软件由平台保障，distill 不格式化磁盘或重建网络。

## 导入与升级

在每个执行节点导入所需镜像：

```sh
cd /absolute/path/gcs-distill2-x86
sha256sum -c SHA256SUMS
python3 scripts/load-images.py --bundle "$PWD"
python3 scripts/preflight.py --shared-root /storage-root-jfs
```

只运行数据任务的节点可对两条 Python 命令加 `--data-only`。导入只使用 `docker load`，核对 SHA256、image ID 和 amd64 架构，不执行 pull。

现有平台分别在对应主机升级，保留原配置和一份回滚二进制：

```sh
python3 scripts/install-binary.py --bundle "$PWD" --component manager --target /root/go/src/gcs-v2/gcs-V2 --service gcs-V2.service
python3 scripts/install-binary.py --bundle "$PWD" --component worker --target /root/go/src/gcs-info-catch-v2/gcs-info-catch-V2 --service gcs-info-catch-V2.service
python3 scripts/install-binary.py --bundle "$PWD" --component distill --target /root/go/src/gcs-distill/bin/gcs-distill-server --service gcs-distill.service
```

worker 构建基线为 Ubuntu 20.04/glibc 2.31，保留 CGO/NVML，未启用 Ascend。不能将其用于更旧 glibc。原 GCS 训练入口资产无需变化；新装平台仍先按 GCS 标准安装包完成基础部署。

新装 distill 使用 `config.example.toml`，填写实际内网地址和目录；systemd 模板指向独立的受保护配置。旧安装只调整新 runtime_image、trainer_image、data_workers 及模型中心地址。`GET /runtime` 是配置目录，不代表已验证所有节点镜像库存。缺少本地镜像会快速失败。

资源选择需要同时升级本包 manager 和每个拟参与训练的 x86 worker，沿用原 Redis 采集配置。新版 worker 上报采集时间与 NVML 进程数；未升级、监控缺失或过期的节点仍可展示，但训练卡禁选，不会将未知占用当空闲。当前现场已升级 `specialai-3090-0`，其他节点纳入蒸馏前也要升级 worker、离线导入两份镜像并完成预检。训练只允许单台服务器上的一张或多张 NVIDIA GPU；不能跨节点，数据阶段不预留 GPU。控制端和 worker 时钟需要同步。

Swagger UI 的脚本和样式已内置，不依赖公网 CDN。包内验收脚本需要 requests，可直接放入数据镜像运行，无需给客户宿主机安装 pip 依赖，命令见包内 `tests/README.md`（源码中为 `tests/integration/README.md`）。

```sh
python3 scripts/cleanup-legacy.py
python3 scripts/cleanup-legacy.py --apply --include-previous-v2
```

旧蒸馏容器即使占用也强制清退；同 image ID 若被其他项目引用则报告阻塞。清理不使用全局 prune，不删除用户共享数据。

## 验收边界

预检镜像烟测使用 `--network none`。`--container-network NAME_OR_ID` 可额外核对实际 worker 网络；`--model /absolute/shared/model` 验证断网加载 tokenizer/config；`--gpu-device 0` 在指定 GPU 进行最小张量检查。真实训练必须使用已分配且显存充足的 GPU。当前交付已经完成平台单卡和双卡的 SFT、DPO LoRA 验收，具体边界见 `docs/easydistill2-x86-verification-20261022.md`。

Agent 模拟工具可离线运行，真实外部搜索以及未安装的图像/视频生成后端未开放。数据质量和学生模型效果分别验收，不伪造 token、成本和模型效果。没有通用断点续跑接口；服务重启后排队任务继续调度，已中断的运行任务标记失败并清理容器，需克隆或新建任务重新执行。

Agent、图文指令、图文 CoT 已使用真实教师完成数据生产到学生 LoRA 的小样本验收。训练镜像为 `gcs/distill-trainer:0.9.3-cu124-vl-x86_64`，内置 qwen-vl-utils，运行时不安装视觉依赖。图文图片需提前上传到当前用户 JFS，数据导出会把附件保存在独立生成数据集目录。

模型权重不包含在程序/镜像包中，交付时需另外复制模型池及 MODEL_PROVENANCE.json；视觉模型为 Qwen2-VL-2B-Instruct，约 4.43 GB。教师服务也应在客户内网提前部署，验收临时服务不能直接当生产服务。模型准备脚本 `x86/prepare-vision-model.py` 只在有网络的构建环境使用，客户现场使用完整模型目录。新包不会联网下载模型、图片或 Python 依赖。

## 用户私有基模

学生基模从当前用户的 `train-center/model-distill/models/base/{model_id}` 加载，列表与详情均要求 uid。用 `scripts/import-model.py --shared-root /storage-root-jfs --uid 369 --source /offline/models/MODEL` 导入完整模型；不修改原始公共模型池。视觉模型包先离线解压，再按用户导入。导入占用该用户配额；首次导入会创建目录。详细树形目录和日志/缓存位置见 `docs/storage-layout.md`。
