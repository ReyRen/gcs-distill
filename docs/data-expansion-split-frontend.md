# 蒸馏数据扩充与划分：前端对接说明

版本：v2.0.7，2026-11-05。平台 API 前缀为 `/modelDistillIpAddress/api/v1`，控制端为 `172.18.29.80:8080/api/v1`。本次提供后端与接口；平台页面由前端补充。

v2.0.8 修复：上述四种指令配方可直接复用单轮 SFT messages 数据，后端自动提取唯一 user 文本；前端无需把 messages 数组映射成 instruction 字符串。`/workflows` 同时提供 seed_input_formats 和 input_note 供提示。多轮、工具、图文不会被静默展平成纯文本。详见 [SFT 种子复用修复说明](sft-seed-reuse-fix-20261105.md)。

## 创建任务怎么补

沿用“项目 → 新建蒸馏任务”，在种子数据和配方之后增加“数据规模”和“数据划分”，放在学生训练、资源选择之前。默认均关闭，避免用户在不知情时增加教师调用量或减少训练数据。

“数据规模”只需一个“扩充种子数据”开关和“目标产出条数”数字框。选择 30 条种子、填 500，表示最终生成 **500 条过滤、去重后的可用数据**，不是每条种子生成 500 条，也不是额外生成 500 条再加上原来的 30 条。关闭时直接处理种子，原生高级配方中的默认扩展阶段也会跳过；筛选可能让产出少于种子数。

页面显示“种子 30 条 → 目标 500 条”。教师调用会增加，耗时与配方、教师响应速度、过滤通过率有关。不要承诺固定倍数或一定成功。基础、高质量、指令增强、类别均衡四种指令配方支持扩充；其他配方暂不支持，按 `/workflows` 的 `supports_data_expansion` 控制开关。它扩展的是新问题指令，不是复制种子或为同一问题更换回答凑数。

原生 `parameters.pipeline[].config.num_output_samples` 不再由前端作为最终目标输入；统一使用下面的 `data_expansion.target_records`。后端会按每轮剩余数量和实际通过率设置候选数，保留原配方的精炼、评分和质量过滤。基础配方开启扩充后使用上游 instruction_expansion + instruct_distill 算子。

“高级设置”可折叠显示候选生成预算、轮数上限、每轮候选上限，普通用户只填目标即可。预算计量的是候选指令数，**不等于教师 API 请求数**；精炼、回答、评分、重试会额外调用教师。

“数据划分”打开后显示训练/验证/测试比例，建议初始 80%/10%/10%，发送小数 0.8/0.1/0.1。训练集用于更新模型参数；验证集在训练时计算 eval loss；测试集留作后续独立评估，本次不会自动测试学生模型。允许 90%/10%/0% 或 90%/0%/10%，训练集必须大于 0，至少保留一个其他集合，总和必须为 100%。

默认按来源、问题和图片分组，随机种子建议 42；来源字段可作为高级项 `group_by`，例如 `metadata.seed_id`。同一组完整进入一个集合，实际条数可能偏离比例。仅有同一个问题的多份回答，或全都使用同一张图片时，可能没有足够独立组，后端会拒绝拆分。自动分组在缺少种子来源标识时返回提示：只能按问题/图片隔离，无法保证原始种子关系隔离；不支持语义相似度识别。

先完成扩充/过滤，再按最终数据划分。目标 500 是三部分合计，不是训练集单独 500。任务创建时还没有最终产物，因此只能预览配置，不能预报准确的划分条数。任务完成后显示真实条数。

## 创建和预览请求

以下片段位于现有 `training_config.distillation` 下，现有 uid、project_id、dataset_id、teacher_model、student_model、资源参数保持原接口。

```json
{
  "version": "2.0",
  "job_type": "advanced_instruct_distill",
  "train_student": false,
  "data_expansion": {
    "enabled": true,
    "target_records": 500
  },
  "data_split": {
    "enabled": true,
    "train_ratio": 0.8,
    "validation_ratio": 0.1,
    "test_ratio": 0.1,
    "seed": 42,
    "group_by": "auto"
  }
}
```

- `POST /workflows/preview`：现有完整创建请求，返回归一化的 `data_expansion`、`data_split` 以及首轮原生 `config`。这是配置预览，不执行教师调用，不创建业务数据。
- `POST /pipelines`：同样的完整请求，配置存入 MySQL 任务快照。
- `POST /pipelines/{id}/start?uid=...`：开始任务，沿用现有接口。
- 关闭时省略该对象，或发送 `{"enabled":false}`；不要仅清空目标数字但保留 enabled=true。
- `/workflows.data.items[]` 增加 `supports_data_expansion`、`supports_data_split`。SFT/DPO 支持划分；仅评分、改写记录等配方不支持。

`resource_request.executor_nodes` 是节点名称字符串数组，最多一个。选择器可以在界面保存节点对象，但提交时必须取 `/resources/available.data.nodes[].name`，不能把整个对象塞进 executor_nodes。只生成数据可发送 `"resource_request": {}`，使用默认离线数据节点；手动指定时例如：

```json
{
  "resource_request": {
    "executor_nodes": ["specialai-3090-0"]
  }
}
```

`selected_resources` 才是学生训练的节点/卡对象数组，形如 `[{"node_name":"节点名称","xpu_indices":[卡号]}]`。关闭学生训练时不提交该项，不需要选 GPU。数据节点从 `data_supported=true` 的节点中选择；训练节点和卡按 `training_supported`、`selectable` 选择。

扩充限制：目标 1～100000；候选预算默认目标的 3 倍、最大 1000000、不得小于目标；每轮上限默认 min(max(目标,4),2000)，范围 1～2000；轮数默认 max(5,3×ceil(目标/每轮上限)) 并限制到 100，范围 1～100。轮数×每轮上限必须覆盖目标。`num_in_context_samples` 不能超过种子条数，配置预览会提前校验。

## 任务详情怎么补

继续使用现有阶段，不增加假的占位阶段。扩充各轮属于“蒸馏数据”，数据划分属于“数据导出”，后续才是可选“学生训练”。

`GET /pipelines/{id}?uid=...` 的 `summary.data_expansion`、以及 stages 中 distill_data 的 `metrics.expansion`，包含：

```json
{
  "target_records": 500,
  "actual_records": 500,
  "current_round": 2,
  "candidates_requested": 715,
  "target_reached": true,
  "stop_reason": "target_reached",
  "rounds": [
    {"round":1,"requested":500,"usable_records":350,"added_records":350,"duplicate_records":0},
    {"round":2,"requested":215,"usable_records":150,"added_records":150,"duplicate_records":0}
  ]
}
```

上面只是字段说明示例，前端必须渲染接口真实返回值。运行中显示“有效 350 / 目标 500 · 第 2 轮”；usable_records 为该轮原生过滤后的有效记录数，added_records 才是扣除跨轮重复并截断到目标后新增的数量。不得直接用 candidates_requested 当产出条数。

触达 `max_rounds` 或 `candidate_budget` 仍未达标时任务失败，提示实际数量和原因；保留已经产出的 JSONL、各轮中间文件和日志，可从“全部产物”预览/下载。**不自动登记成已完成的数据资产，不执行后续学生训练。** 容器异常同样失败。可以复用配置、调整质量阈值/预算后创建新任务。

成功时 `summary.data_split`、dataset_export 的 `metrics.data_split` 返回真实条数、独立组数、配置、警告、源文件 SHA256 与划分 ID；`output_manifest.split_id` 可用于后面的数据集接口。运行产物中包含 `data/exported/splits/{id}/train.jsonl`、validation.jsonl、test.jsonl（禁用的集合不生成文件）和 manifest.json。`data/exported/expansion.json` 记录扩充过程。

日志沿用现有接口：运行时 stage.container_id/log_path 会切换到当前轮；完成后下载日志包含各轮归档。前端若使用容器日志 WebSocket，轮次切换后应读取新的 stage.container_id 并重连；也可以沿用 HTTP 日志刷新。

学生训练的 stage.metrics 返回训练框架实际写出的 `train_loss`、`eval_loss` 和运行时间（若产出）；启用了验证集但没有有效、有限的 eval_loss 时会报错，不应将这种情况显示为验证成功。

## 已有数据集单独划分

“我的数据集”中，蒸馏生成的 SFT/DPO 数据行增加“划分数据”，打开同一套比例配置面板。源数据只读；可保存多个不可变划分快照。重复提交相同源内容、比例、种子和分组方式，返回同一个 ID，不重复写文件。删除整个生成数据集时，其下划分文件随原资产一并清理。

| 功能 | 请求 |
|---|---|
| 预览实际条数，不写文件 | `POST /datasets/{id}/splits/preview?uid=369` |
| 保存划分 | `POST /datasets/{id}/splits?uid=369` |
| 已保存的划分 | `GET /datasets/{id}/splits?uid=369` |
| 指定划分详情 | `GET /datasets/{id}/splits/{split_id}?uid=369` |
| 预览子集 | `GET /datasets/{id}/splits/{split_id}/partitions/{train|validation|test}/preview?uid=369&limit=10` |
| 下载子集 | `GET /datasets/{id}/splits/{split_id}/partitions/{train|validation|test}/download?uid=369` |
| 交给训练页的参数 | `GET /datasets/{id}/training-config?uid=369&split_id={split_id}` |

两个 POST 的请求体均为上面的 `data_split` 对象本身，不包 `data_split` 外壳。需从登录态传 UID，不提供 UID 输入框。

预览和保存都返回 `{code:200,data:{id,dataset_id,config,source_sha256,total_records,total_groups,partitions,warnings,created_at}}`。partitions 中 train/validation/test 分别包含 `records`、`groups`、`actual_ratio`。保存后非空集合另有 `preview_path` 和 `download_path`，是相对 API 前缀的路径，包含 uid；直接拼接平台 API 前缀，不能绕到网站根目录。预览尚未保存，不会返回可下载链接。

预览成功后显示例如“训练 400 / 验证 50 / 测试 50”，同时展示 warnings，再允许“保存划分”。不要根据前端四舍五入代替后端结果。400 表示配置错误；404 表示不存在或无归属权限；422 表示不支持的配方、数据异常、独立组不足或划分失败。禁用的集合不可下载。

列表返回 `data.items` 和 `data.default_split_id`。默认划分是该数据集生成时的任务配置；后续手工划分不会暗中替换默认值。

training-config 在传入 split_id 时只将 train.jsonl 放到 `dataset_info.gcs_distilled`，validation.jsonl 放到 `gcs_validation`，并返回 do_eval、eval_dataset、eval_strategy 等训练配置；测试集不进入 dataset_info，也不会自动评估。省略 split_id 时，若生成任务配置了划分则使用其默认划分，否则沿用全量训练。指定无效/损坏的划分返回错误，禁止悄悄退回全量数据。平台“去训练”跳转仍由训练页面团队接通，本次接口不创建外部训练任务。

## 持久化、离线与边界

用户目录无需提前创建，继续使用 `/storage-root-jfs/user-{uid}/train-center/model-distill/`。生成数据资产保存到 `datasets/generated/{dataset_id}/`，划分位于同目录的 `splits/{split_id}/`，自带 manifest.json 和各 JSONL；运行目录也保留独立快照。删运行目录不会让生成资产的划分失效。图文行保留私有资产 images 的引用，JSONL 下载本身不打包图片。

任务扩充/划分参数和阶段统计存于既有 MySQL JSON 字段；手工划分是 JFS 内不可变文件快照，通过清单枚举，无需新建数据库表或手工迁移。源文件指纹针对规范化 JSONL 内容计算，顺序计入指纹。

本次全部逻辑在控制端执行，继续使用已准备的 EasyDistill2 和 LLaMA-Factory 镜像，不增加依赖、不联网下载，不需要新部署教师服务；沿用平台选择的现有教师。扩充不申请新的教师 GPU，学生训练仍按现有单机多卡选择。控制端与 worker 需保持相同 JFS 挂载路径和写权限。

数据划分是数据管理能力，不等于学生模型质量保证。eval loss 也不等同于最终业务测试成绩。
