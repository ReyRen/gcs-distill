# EasyDistill 2 x86：页面设计与前端交接

核对日期：2026-10-22。依据本仓库、gcs-s3 的目录机制、GCS 容器协议和平台实际页面。交互 mock 使用标注的合成示例数据，不连接生产 API。

## 实际页面发现

- 已查看项目列表、新建项目、项目详情、数据集页签、流水线列表和创建入口。
- 任务藏在项目详情弹窗内，再套创建弹窗；窄屏有横向溢出，创建标题显示“创建流水线00”。
- 项目创建提前要求学生模型；创建流水线仍是“基础信息 → 训练参数 → 资源选择”，缺少数据目标、配方、质量策略及数据产物。
- 客服项目详情显示 10 条任务、8 次成功。已核对数据库和 summary 接口，实际只有一条失败任务。必须使用真实统计。
- API 地址占位符以 `/chat/completions` 结尾；新版提示基础地址 `/v1`，后端兼容剥除旧后缀。

## 业务边界和存储

| 对象 | 当前机制 | 页面设计 |
|---|---|---|
| 用户 | 项目、数据、任务均保存 UID，创建任务验证三者一致 | 从登录态传递，不提供任意 UID 输入框 |
| 项目 | 业务分组、可选默认教师 | 组织任务和教师配置；本次配方保存到任务 |
| 数据集 | 属于用户，独立于项目 | 同用户多项目复用，不能把项目当作归属 |
| 教师 | 模型中心服务或兼容 API | 2.0 选择推理服务；本地权重须先部署为服务 |
| 学生 | 从共享基模目录选择 | 选择器始终可见，开启训练后必选，不输入任意服务器路径 |
| 调度 | distill → gcs-v2 → worker | 数据生产与 GPU 训练分别展示状态 |
| 评估 | 生成数据评分与学生模型效果不同 | “样本与质量”看数据，学生效果进入独立测评 |

后端派生的目录：

```text
/storage-root-jfs/user-{uid}/train-center/model-distill/
├── datasets/
│   ├── candidates/             # 受控候选登记
│   ├── uploaded/{dataset_id}/  # 上传数据
│   └── generated/{dataset_id}/ # 独立生成资产
└── projects/{project_id}/runs/{pipeline_id}/
    ├── configs/                # 本次参数快照，含密钥文件不开放下载
    ├── data/seed/              # 保留完整字段的 JSONL 输入
    ├── data/intermediate/      # 官方算子中间产物
    ├── data/exported/          # 最终数据与清单
    ├── logs/
    └── models/checkpoints/     # 只有学生训练产生模型
```

只读基模池为 `/storage-root-jfs/train-base-models`。gcs-s3 创建稳定目录，distill 按需创建运行目录。主流程显示资源名、来源和用途；路径放“存储与版本信息”。

## 页面结构

保留现有蓝色顶栏、左栏、工作标签和组件风格。蒸馏模块采用三个一级页签：

1. **项目管理**：业务信息、默认教师和实际统计，不强制学生模型。
2. **蒸馏任务**：名称、目标、配方、状态、产物条数、时间。查看进入整页工作区，取消嵌套弹窗。
3. **数据集管理**：上传、候选登记、蒸馏生成；格式、记录数、来源任务、预览和训练入口。

任务详情为“运行概览 / 样本与质量 / 日志与产物”。UUID 放详情或复制操作，不作为列表主标题。

| 创建步骤 | 配置内容 | 行为 |
|---|---|---|
| 目标与学生 | 仅生成数据 / 生成数据并训练学生、学生基模、数据配方 | 第一屏始终展示学生基模字段，仅生成数据时禁用并说明原因 |
| 数据与教师 | 独立数据集、预览、字段映射、推理服务/API | 校验对应输入结构和附件可访问性 |
| 质量与产物（开启训练时为质量与训练） | 原生配方实际支持的评分、过滤或采样规则；可选训练参数与资源 | 不同目标显示不同参数，保留第一步已选学生基模 |
| 确认提交 | 输入、教师、配方、规则及流程快照 | 先创建再启动，区分 pending 与运行中 |

SFT 用质量阈值；CoT 用逻辑正确性、RV/CD 和难度分组；DPO 用候选数、评分差、优选/差选；Agent 用步骤数、轨迹数和工具模式；多模态须检查图文输入和教师能力。

第一步使用两个明确的任务目标：“仅生成蒸馏数据”和“生成数据并训练学生”，默认前者。学生基模选择器始终可见，前者禁用并显示无需基模，后者必须从本地共享模型池选择；不能再藏在第三步或高级配置中。第三步配置 LLaMA-Factory 镜像、模板、LoRA/全量参数和 GPU 节点/卡。确认页与任务详情展示已选基模名称，修改目标、配方或前后切换步骤不能丢失选择。

学生列表来自 `GET /api/v1/models/student`。模型项保存 ID，名称用于展示；提交到 `training_config.distillation.student_model.model_id`，后端解析共享池内路径。学生直接加载本地权重进行训练，无需部署推理服务。列表为空时显示“暂无可用学生基模，请先将模型纳入共享模型池”，加载失败可重试，不能退化为要求填写服务器路径。Mock 的模型为明确标注的示例，不能当作真实库存。

本次教师配置保存为 `training_config.distillation.teacher_model` 快照；省略时从项目默认教师取得。学生使用 `student_model.model_id`，后端从共享模型目录解析路径。教师或学生的每次选择不能通过修改项目默认值来间接改变其他任务。训练请求还必须提交正数 `num_train_epochs`、`learning_rate`；当前选择单个训练节点，可选择该节点的多张 GPU。

## 前端接口映射

| 需求 | 接口 |
|---|---|
| 配方目录/版本/输入字段 | `GET /api/v1/workflows` |
| 不创建任务的配置预览，隐藏密钥 | `POST /api/v1/workflows/preview` |
| 项目真实任务统计 | `GET /api/v1/projects/{id}/summary?uid=...` |
| 输入数据预览、字段和实际条数 | `GET /api/v1/datasets/{id}/preview?uid=...&limit=10` |
| 创建/启动/取消任务 | 现有 `/pipelines`、`/{id}/start`、`/{id}/cancel` |
| 外层阶段状态 | `GET /api/v1/pipelines/{id}/stages` |
| 中间及最终数据 | `GET /api/v1/pipelines/{id}/artifacts?uid=...` |
| 预览/下载 | `GET /api/v1/pipelines/{id}/artifacts/preview?uid=...&path=...`，下载加 `download=true` |
| 日志 | 沿用现有阶段 `/logs` 和 `/logs/ws` |
| 学生基模/GPU | 沿用 `/models/student`、`/resources/available` |

外层阶段为 `teacher_config → dataset_build → distill_data → dataset_export`；开启训练再加 student_train，不能写死六阶段。官方生成、评分、筛选在 distill_data 内运行；内层进度根据实际日志与中间产物展示，不能把推测进度标成完成。

数据生产为 GCS CPU 容器任务，教师推理使用已有服务。`resource_request.executor_nodes` 选择已加载镜像的数据节点；训练 GPU 仍用 `selected_resources`。

教师服务选择调用 distill 的 `GET /api/v1/teachers?uid=...`；提交 `teacher_model.service_id` 即可，后端校验归属、在线状态并从模型中心取得连接信息和凭据。手工 API 配置提交 `endpoint`、`model_name`、`api_secret_ref`。`POST /api/v1/teachers/test` 会验证鉴权、模型发现和少量聊天推理；不会自动证明视觉能力。执行地址必须从 worker 可达，不得使用浏览器代理相对地址或 localhost。密钥只提交一次，回包中的 `********` 不能作为新密钥提交。

最小创建请求：

```json
{
  "uid": 369,
  "project_id": "当前用户的项目 ID",
  "dataset_id": "当前用户的独立数据集 ID",
  "training_config": {
    "distillation": {
      "version": "2.0",
      "job_type": "instruct_distill",
      "train_student": false
    }
  },
  "resource_request": {}
}
```

369 是本次授权账号的示例 UID，正式实现从身份获取。版本、配方与参数通过既有 training_config JSON 的 distillation 子对象持久化，无需新增业务表。质量增强使用 advanced_instruct_distill，阈值保存到原生 pipeline 的 quality_filter.config.min_scores；parameters 与官方模板深合并，路径及教师由后端确定。完整合同见 OpenAPI。

## 实现边界与验收

### 本次复查结论

Mock 覆盖核心主流程，可用于页面和交互交接；它不是已经接入生产 API 的前端，也不是 EasyDistill 全部配方的完整编辑器。前端必须结合本说明和 OpenAPI 实现，不能直接复制演示请求或演示模型库存。

| 能力 | Mock 覆盖情况 | 实现时的依据与边界 |
|---|---|---|
| 项目、用户与数据集的关系 | 已表达 | 数据属于用户，项目仅组织业务；项目创建、编辑和真实统计仍需接现有 API |
| 教师推理服务与兼容 API | 已表达选择入口 | 模型中心解析 worker 可达的连接信息；示例请求没有完整演示密钥和服务发现 |
| 核心数据目标 | 指令、CoT、DPO、Agent、多模态五类 | 当前可选 11 个配方，后端目录为 15 个；独立 instruct_eval/cot_eval、search_agent_distill、mm_cot_distill 未在 Mock 单独展开 |
| 输入数据与存储 | 已表达独立数据集、来源与目录 | 上传、候选登记为动作提示，字段映射与图片附件检查需按真实格式实现 |
| 数据质量 | SFT 阈值和 DPO 评分差可交互 | DPO 按 chosen/rejected 分差筛选，不能按单个回答的有用性分筛选；其他配方参数按原生 YAML 实现 |
| 学生训练 | 已表达基模、镜像、模板、微调方式、参数、GPU | 基模选择已联动；其余训练控件主要表达布局，正式值须完整绑定实际请求，训练镜像与资源不使用示例占位值 |
| 任务生命周期 | 已补充待启动、排队、准备、运行、失败、取消、完成的状态映射 | 详情可演示启动、取消；正式调用 start/cancel 并读取状态，不以按钮点击推断成功 |
| 执行阶段 | 已修正为四个外层阶段，可选第五个学生训练 | 生成、评分、筛选是 distill_data 内层步骤，不能把它们当成独立 StageRun |
| 失败与部分成功 | 已补充学生训练失败、数据仍可用的状态 | 错误归属实际失败阶段，保留成功导出的数据；不提供不存在的原地重试或断点续跑 |
| 数据产物与样本 | 已表达预览、下载、来源和复用 | 无正式产物时不展示可下载的数据；演示样本不代表真实 CoT/Agent/视觉数据结构 |
| 学生模型产物 | 已补充权重状态与训练日志 | 从 student_train 的 output_manifest.checkpoint_path 读取位置；已提供单文件和模型 ZIP 下载；不自动注册模型中心 |
| 学生效果评估 | 明确显示尚未测评 | 需与独立测评模块联调，不能用数据质量评分代替模型效果 |

复查修正：任务不再统一显示“待提交”；DPO 预览与 SFT 分数分开；训练权重与数据集分别展示；无数据时禁用产物相关动作；运行流程按实际四/五阶段展示。运行、失败和训练完成均为清楚标注的合成状态示例，不能写入生产统计。

### 前端上线前必须联调

1. 所有配置控件保存到本次任务，前后切换不丢失；JSON 示例仅作结构对照，不作为完整提交体。
2. 教师服务列表、学生模型池、训练镜像、资源卡、数据集列表使用真实接口，并提供加载中、为空、错误和重试状态。
3. 校验配方产物类型与学生训练兼容性：records/scored_records 不能直接接训练；文本学生不能因选择视觉目标就被认定为视觉基模。
4. 外层状态和日志读取后端；取消后等待后端确认；旧任务仅展示已存记录，禁止启动；所有新任务使用 2.0。
5. 样本预览按实际 messages、chosen/rejected、工具消息和图片字段渲染；需要视觉附件鉴权时通过现有文件机制处理。
6. 模型产物使用已提供的下载接口；自动注册模型中心和学生效果测评属于独立模块，不开放无接口的按钮。

训练请求的必要映射如下，其他项目、数据与教师信息也必须纳入同次创建请求：

| 页面字段 | 请求字段 |
|---|---|
| 学生基模 | training_config.distillation.student_model.model_id |
| 教师连接快照 | training_config.distillation.teacher_model |
| 是否训练 | training_config.distillation.train_student |
| 训练镜像 | training_config.distillation.trainer_image |
| 对话模板 | training_config.distillation.model_template |
| LoRA / 全量 | training_config.distillation.finetuning_type = lora / full |
| 轮数、学习率、每卡批量 | training_config.num_train_epochs / learning_rate / per_device_train_batch_size |
| 单节点及卡号 | resource_request.selected_resources[].node_name / xpu_indices |
| 数据执行节点，可省略沿用服务配置 | resource_request.executor_nodes |

### 已有运行验收与尚待验证项

- 任务名保存为 `training_config.distillation.name`，随本次配置持久化。历史任务回退显示创建时间和短 ID，不能只存浏览器内存。
- 旧任务没有 distillation 子对象，返回 summary.retired=true，仅保留历史记录且不能启动；旧执行代码和镜像已清退。
- JSON 数组按对象计数；JSONL 严格逐条解析，非法行不静默丢弃。TXT 不作为 2.0 可直接运行的格式推荐。
- 不丢失 messages、images、工具轨迹、偏好对及 metadata，不能一律压成 instruction/output 两列。
- 无产物不能显示成功；数据生成失败不叫“模型训练失败”。
- 预览最多 100 条，不能据此推算全量评分分布。`/quality` 提供按中间产物全量扫描的评分统计；token 和成本返回 null，禁止填演示数字。
- Agent 默认模拟工具，不标注为已接真实业务系统。多模态须验证视觉服务、附件和挂载。
- 图像/视频生成等尚未安装额外依赖的能力不能直接开放为“已可运行”。
- 当前保留中间及恢复文件，没有通用断点续跑 API，不提供无效续跑按钮。
- 目录内 15 个配方经过配置编译检查。基础指令、CoT、质量筛选和 DPO 进行镜像及实际 GCS 验收，其余需逐项做模型效果验收。
- 可选 LLaMA-Factory 训练已有独立配置和调度通道，GPU 训练、学生效果测评须独立验收。

`ui/gcs-distill2-mock.html` 可直接打开。支持页签、任务详情、样本筛选、DPO 比较、创建向导、数据/训练切换和本地示例提交；上传、下载、跨模块训练只提示业务动作。前端应复用现有组件实现，不替换整个平台导航。

## 后端实施后的接口补充（2026-10-22）

所有 `/pipelines/{id}` 的读取、启动、取消、阶段、日志、WebSocket 和产物接口均必须携带 `?uid=...`。UID 必须由平台认证网关注入，不能信任用户自行填写。跨用户资源返回 404。

| 页面操作 | 已实现接口与返回依据 |
|---|---|
| 全部任务、项目内任务与筛选 | `GET /pipelines?uid=...&project_id=可选&name=...&status=...&job_type=...`，返回真实分页 total |
| 项目/数据名称筛选 | `GET /projects?uid=...&name=...`；`GET /datasets?uid=...&name=...&source_type=...` |
| 教师服务目录与测试 | `GET /teachers?uid=...`；`POST /teachers/test`，body 为 uid、teacher_model |
| 镜像、数据节点、训练模式和模板 | `GET /runtime`；这些是已配置目录，节点镜像实际状态由离线部署预检确认 |
| 学生基模是否可选 | `/models/student` 的 ready、readiness_error、supports_vision、suggested_template；ready=false 禁用训练选择 |
| 字段映射 | `training_config.distillation.field_mapping`，键为规范字段，值为种子数据原字段，如 {"instruction":"question"} |
| 列表/详情统计 | PipelineRun.name、summary，input_records/output_records/output_dataset_id/has_model_weights；无产物不伪造统计 |
| 数据来源 | Dataset.lineage：pipeline、seed_dataset、job_type、revision |
| 数据集下载 | `GET /datasets/{id}/download?uid=...` |
| 产物单文件下载 | `GET /pipelines/{id}/artifacts/download?uid=...&path=...` |
| 学生权重下载 | `GET /pipelines/{id}/model/download?uid=...` 返回 ZIP；尚无权重返回 404；LoRA 产物为适配器，使用时需要对应基模 |
| 全量数据质量 | `GET /pipelines/{id}/quality?uid=...`；每个 scored_artifacts 单独汇总 count/min/max/mean，不跨重复中间文件累加 |
| 已结束任务日志 | 原日志接口继续可用，读取共享存储归档；完成后容器被回收 |

路径省略统一前缀 `/api/v1`。状态为 pending → scheduled → running → succeeded/failed/canceled；不要把中间算子日志当成外层阶段。缺少本地镜像会报错，所有蒸馏容器均禁止联网拉取镜像或模型。教师服务通过内网 HTTP 调用，学生权重必须预先完整放入共享模型池。

### 复用及跳转训练

`POST /pipelines/{id}/clone?uid=...`（可选 body：`{"name":"副本名称"}`）从服务端配置快照复制为新的 pending 任务，不启动；密钥不会经过前端。副本重新校验教师、数据和学生模型，不承诺复用已下线服务。

“去训练 / 使用此数据集训练”跳转现有模型训练页面，并携带 distill 数据集 ID。训练页面调用 `GET /datasets/{id}/training-config?uid=...`，获取 SFT/DPO stage 和 LLaMA-Factory dataset_info，加入现有训练创建流程。这个接口不创建训练任务、不复制数据、不注册模型。源数据仍在当前用户共享目录，训练模块挂载用户目录并复用已有调度。scored_records 和普通 records 返回 422，不能误作可训练数据。

资源接口中的 available/enable_xpu_indices 表示平台允许调度，不代表当前物理显存空闲；卡占用与显存展示继续使用平台实时监控数据，不能把“启用”渲染为“无人使用”。

服务重启会恢复尚未执行的 scheduled 任务；已经运行且中断的任务标记失败、回收容器，保留成功的数据资产。没有断点续跑。

### 原生默认参数与质量图表

`GET /workflows` 每项返回 `default_parameters`。切换配方时用它初始化表单，再修改对应生成、评分、过滤参数，提交到 `training_config.distillation.parameters`。数组会整体替换：编辑阈值时必须保留完整 pipeline 算子数组，不能只提交一个过滤算子。后端控制教师和路径，不提供它们作为可编辑默认值。

`GET /pipelines/{id}/quality` 每个指标提供 histogram 和 histogram_bin_width=0.1，可绘制全量评分分布；桶标签为区间下界，other 表示超出桶数上限的长尾。count/min/max/mean 为精确聚合，不把直方图区间当作精确任意阈值过滤结果。DPO 的 candidate_score、chosen_score、rejected_score、preference_margin 分开显示。每个中间产物独立汇总，不能把同一批样本的多个产物累计当作总样本数。

“全部”筛选不发送对应 status/job_type/source_type 值，不传字面字符串 all。
