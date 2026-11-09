# 小样本验收、生成进度与提前校验

版本 v2.0.9，2026-11-09。平台 API 前缀 `/modelDistillIpAddress/api/v1`。

## 种子数量必须明确

创建/复用任务的 `training_config.distillation.seed_limit` 控制本次输入数量。0 或省略表示全量；5 表示取源文件前 5 条，源文件不修改、不额外登记数据集。若文件不足 5 条，使用其实际条数。整个文件仍检查 JSON 语法，字段校验只作用于本次选中的记录。

这和 `data_expansion.target_records` 不同：seed_limit 是输入数量，target_records 是扩充、过滤后的输出目标。不要用前端预览条数、分页条数或“显示 5 条”代替 seed_limit。

`POST /workflows/preview` 返回 `data.seed_selection`：

```json
{"source_records":30,"selected_records":5,"seed_limit":5,"method":"first_n"}
```

前端应在提交前显示“原数据 30 条，本次前 5 条”。执行后 `summary.source_records` 是源数据总数，`summary.input_records` 是实际进入本次任务的种子数；不应把二者混用。

## 提前检查数据与配方

选中数据集后，先调用 `GET /datasets/{id}/compatible-workflows?uid={uid}&seed_limit=5`。服务端一次扫描源文件，按本次选中的记录检查全部 15 种配方，返回 seed_selection 和 items。每项含 job_type、name、compatible、reason、invalid_record、input_fields、output_format。前端将 compatible=false 的配方置灰，并显示第一条不匹配记录及原因；不能只根据扩展名或前 10 条预览判断。全量模式会检查所有记录。

此接口按默认字段名检查；如果用户想使用自定义列，保留“配置字段映射”入口，完成后用下面的单配方校验接口复核，通过后再允许选用。更换数据集后重新检查，不能自动沿用上一个数据集的可用类型。含 images 的数据不会被纯文本任务静默丢图；质量评估需要实际存在的回答，空 messages 不视为合格。

选择数据集、切换任务类型、修改字段映射或 seed_limit 时立即调用：

`POST /datasets/{id}/validate?uid={uid}`

```json
{"job_type":"instruct_distill","seed_limit":5,"field_mapping":{}}
```

不需要先选教师、学生或 GPU，也不调用模型、不创建任务。成功返回 valid=true 和 seed_selection；字段不匹配返回 422、valid=false、具体记录序号和原因。前端在检查完成前禁用“下一步”，失败时显示原因并提供字段映射/更换数据集入口。过期请求结果不能覆盖新选择；使用 AbortController 或请求序号抑制竞态。

该检查负责源 JSON 和选中记录字段，不检查教师可达性、图片实际可读取性或 GPU 空闲状态。完整配置仍调用 `/workflows/preview`；创建、执行时继续校验，不能绕过后端规则。

## 教师请求与重试

`teacher_model` 增加 `retry_attempts`，为首次请求之后的重试次数，范围 0～3；0 表示不重试。默认并发 1、max_tokens 2048、timeout_seconds 120、retry_attempts 1，显式的并发和长度继续尊重用户配置。小样本验收先使用并发 1。对于当前带思考输出的 qwen3-vl:32b，保留 max_tokens 2048；实测 512 会出现推理内容耗尽预算、正文为空而被过滤。非思考模型可另外验证较短上限，不能只为速度牺牲有效输出。

推荐默认值也可读取 `/runtime.data.teacher_defaults`。教师是已有共享服务，任务并发并不代表教师能并行处理相同数量的请求。例如服务端 `OLLAMA_NUM_PARALLEL=1` 时，任务并发 4 会产生排队。不要在前端默认写死 4。

后端固定关闭 SDK 层重试，由支持该参数的原生生成算子执行 retry_attempts，避免额外的嵌套重试。部分上游评分器没有透传 retry_attempts，仍使用其内部策略；不能将该字段理解为所有配方的每次评分均有相同重试次数。如果原生算子连续记录 3 次超时且中间没有成功响应，控制端停止本任务的数据容器并将任务标为失败；该保护覆盖评分阶段，已落盘日志/中间文件保留，共享教师服务不停止。检查周期约 5 秒。阈值针对当前数据容器，扩充切换容器后统计重新开始。

## 进度显示

沿用 `GET /pipelines/{id}?uid=...` 的 `summary.generation`，或 stages 中 distill_data 的 `metrics.generation`，每 3～5 秒刷新：

```json
{
  "container_id":"实际容器名称",
  "phase":"generate",
  "successful_requests":3,
  "failed_requests":0,
  "retry_attempts":1,
  "timeout_attempts":1,
  "consecutive_timeout_attempts":0,
  "circuit_open":false,
  "queue_depth":null,
  "updated_at":"实际更新时间"
}
```

以上只是字段示意，生产必须渲染接口返回的真实值。统计来自 worker 日志的新增完整行，不用运行时间猜进度：

- successful_requests 是教师 HTTP 200 响应次数，不等于最终数据条数。高级配方会进行精炼、生成、评分等多次请求。
- failed_requests 是算子耗尽尝试后的失败请求次数；retry_attempts 是实际记录的重试次数。
- invalid_results 是原生构建器记录的无效响应丢弃次数；built_samples 是原生构建器报告的有效 SFT 条数，尚未报告时为 null。HTTP 成功但正文为空时可以在这里识别，不能显示为“样本全部成功”。
- timeout_attempts 是超时尝试总数；连续超时计数在成功响应后清零，但熔断状态一经触发就保持。
- queue_depth=null 表示不能观测共享教师真实队列，前端显示“未知”，不得编造排队数量。
- 尚无日志时 generation 可缺省；历史任务不回填猜测数据。扩充每轮的 container_id 会变化，统计按当前容器显示；最终样本数使用 output_records，扩充累计进度仍使用 data_expansion。

推荐展示：“教师生成 · 成功请求 3 / 失败请求 0 · 重试 1”，超时后突出提示，不要一直只有“运行中”。耗时从真实 stage.started_at 计算。学生训练状态单独展示。

开始任务时会检查所选 GPU 是否空闲，当前已占用则立即拒绝，避免先花几十分钟生成数据再发现无法训练。数据阶段不预占 GPU，长任务中资源仍可能变化；进入学生训练前会再次检查，失败时保留导出数据。前端选卡只能勾选 selectable=true 的卡。

本次不增加镜像或外部依赖、不修改共享 Ollama 配置、无需数据库迁移。参数/统计使用既有 MySQL JSON 字段，原始数据和运行目录继续按 UID 使用 JFS。

另修正新任务 started_at 使用数据库会话时区导致与 created_at/阶段时间相差 8 小时的问题，统一使用控制端时间入库。历史记录未批量改写；历史任务耗时优先根据阶段时间计算。
