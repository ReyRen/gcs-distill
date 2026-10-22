# 蒸馏文件目录与归属

所有蒸馏业务文件均归当前 UID，统一前缀为 `/storage-root-jfs/user-{uid}/train-center/model-distill/`。
控制端与执行节点必须挂载同一份 JFS，绝对路径一致。目录按首次查询、上传、导入或运行自动创建，不需要提前逐项建目录。

```text
model-distill/
├── models/
│   ├── base/{model_id}/                      # 该用户完整学生基模，只读挂入容器
│   │   ├── config.json / tokenizer* / processor* / 权重分片
│   │   ├── MODEL_PROVENANCE.json             # 有上游来源时保留
│   │   └── MODEL_IMPORT.json                 # 导入 UID、来源、逐文件大小与 SHA256
│   └── .imports/                            # 导入暂存，完整校验后原子发布
├── datasets/
│   ├── candidates/                          # 候选原始数据，仅登记引用
│   ├── uploaded/{dataset_id}/               # 用户上传的种子数据
│   └── generated/{dataset_id}/
│       ├── dataset.jsonl                    # 可跨项目复用的最终数据资产
│       └── images/                          # 图文附件，内容哈希命名
└── projects/{project_id}/runs/{pipeline_id}/
    ├── configs/                             # 本次参数、教师配置、训练配置与字段映射
    ├── data/
    │   ├── seed/input.jsonl                  # 本次输入快照
    │   ├── intermediate/                    # 评分、筛选、轨迹、原生图文等中间数据
    │   └── exported/                        # 本次最终数据与 manifest.json
    ├── models/checkpoints/
    │   ├── adapter_model.safetensors 等      # 最终 LoRA；全量训练为完整模型权重
    │   └── checkpoint-{step}/               # 训练框架产生的阶段检查点
    ├── logs/{stage}/
    │   ├── events.log                       # 各阶段 started/succeeded/failed/canceled 事件
    │   ├── log.txt                          # worker 持久化的容器标准输出/错误
    │   ├── archived.log                     # 容器释放前归档的日志
    │   └── trainer/                         # 训练框架 logging_dir
    ├── cache/                               # Hugging Face、datasets、Torch、Triton 等缓存
    └── tmp/                                 # 容器 TMPDIR
```

模型目录中实际生成哪些权重和处理器文件，由模型与训练方式决定；示意文件名并非要求每个模型都有。训练框架的 trainer_state、trainer_log 等训练记录也可能随检查点保存。程序运维日志由 systemd 管理；与任务有关的阶段、容器日志落在上述用户目录。

## 学生基模与离线导入

`GET /api/v1/models/student?uid=369` 和详情接口均要求 UID，只列出当前用户 `models/base` 中的模型。创建/预览任务时，服务端按 UID 与 model_id 重新解析路径；不接受其他用户、公共池或符号链接指向的基模。基模缺失返回空列表或明确错误，不自动下载，也不向每个用户自动复制所有模型。

管理员将准备好的完整模型离线导入指定用户：

```sh
python3 scripts/import-model.py --shared-root /storage-root-jfs --uid 369 \
  --source /offline/models/Qwen2-VL-2B-Instruct
```

工具不联网，不覆盖已有模型；复制时计算校验值并核对来源清单，完整后才进入可选目录。失败会清理本次暂存；目标已存在时需管理员另行处理版本。模型池中的原文件不会移动、链接或删除，其他 SFT/推理项目可继续使用原路径。教师权重继续由模型中心管理，蒸馏只调用对应的内网服务。

## 生命周期与权限

- UID、项目 ID、运行 ID、数据资产 ID 分层隔离；同名模型属于各自用户，重复任务不覆盖原运行。
- 用户基模、数据资产按需只读挂载；本次运行目录可写。HOME、TMPDIR、HF_HOME、HF_DATASETS_CACHE、TORCH_HOME 等均指向本次运行下的 cache/tmp。
- 含密钥的 configs 目录为 0700，配置文件为 0600；配置和缓存不进入产物下载。模型 ZIP 只含允许的权重/元数据。
- 容器退出不删除 JFS 数据、检查点或日志。生成数据及图片有独立资产目录，不依赖运行目录存活。
- 数据资产删除仍遵守数据库引用检查；登记候选的源文件不随登记记录删除。项目删除沿用现有元数据删除行为，不递归清理 JFS 文件。任何保留期/历史文件清理需显式制定，不在后台悄悄删除权重或日志。
- 平台需保障挂载、写权限、用户容量与文件数配额。gcs-s3 默认用户容量已调整为 1TiB（1024GiB）；容量由 JFS 统一管理，蒸馏服务不自行修改配额。
