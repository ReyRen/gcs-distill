# EasyDistill 2 x86：页面设计与前端交接

核对日期：2026-10-22。依据本仓库、gcs-s3 的目录机制、GCS 容器协议和平台实际页面。交互 mock 使用标注的合成示例数据，不连接生产 API。

删除项目时，若仍有 scheduled、preparing、running 任务，后端返回 409。前端应提示先取消并等待任务结束，再删除；不要把失败当成删除成功。数据库事务同时锁定项目及任务，避免与启动操作竞态。

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
| 项目 | 当前平台创建任务的必需归属、可选默认教师 | 作为外层入口；进入项目后管理任务，本次配方保存到任务 |
| 数据集 | 属于用户，独立于项目 | 同用户多项目复用，不能把项目当作归属 |
| 教师 | 模型中心服务或兼容 API | 2.0 选择推理服务；本地权重须先部署为服务 |
| 学生 | 从当前用户基模目录选择 | 选择器始终可见，开启训练后必选，不输入任意服务器路径 |
| 调度 | distill → gcs-v2 → worker | 数据生产与 GPU 训练分别展示状态 |
| 评估 | 生成数据评分与学生模型效果不同 | “样本与质量”看数据，学生效果进入独立测评 |

后端派生的目录：

```text
/storage-root-jfs/user-{uid}/train-center/model-distill/
├── models/base/{model_id}/    # 当前用户完整学生基模
├── datasets/
│   ├── candidates/             # 受控候选登记
│   ├── uploaded/{dataset_id}/  # 上传数据
│   └── generated/{dataset_id}/ # 独立生成资产
└── projects/{project_id}/runs/{pipeline_id}/
    ├── configs/                # 本次参数快照，含密钥文件不开放下载
    ├── data/seed/              # 保留完整字段的 JSONL 输入
    ├── data/intermediate/      # 官方算子中间产物
    ├── data/exported/          # 最终数据与清单
    ├── logs/{stage}/            # 生命周期、容器和训练日志
    ├── cache/                   # HF/Torch 等缓存
    ├── tmp/                     # 运行临时文件
    └── models/checkpoints/     # 只有学生训练产生模型
```

用户私有基模目录为 `/storage-root-jfs/user-{uid}/train-center/model-distill/models/base`。gcs-s3 创建稳定目录，distill 按需创建运行目录。主流程显示资源名、来源和用途；路径放“存储与版本信息”。

## 页面结构

保留现有蓝色顶栏、左栏和页面导航。以项目作为任务的外层容器，按“项目列表 → 项目工作区 → 任务详情”逐层聚焦。默认只显示当前层级的主要操作，参数和技术信息按需展开。蒸馏模块保留两个全局入口：

1. **蒸馏项目**：默认首页。先选项目，进入项目工作区后显示该项目的任务和“新建任务”；项目设置管理名称与可选默认教师，不强制学生模型。
2. **我的数据集**：用户级公共资产入口。上传、候选登记和蒸馏生成使用同一列表；提供预览、下载、用于蒸馏或去训练入口。

当前 `pipelineService.validatePipeline` 要求非空 `project_id`，校验项目存在且 UID 一致；教师配置缺省时取项目默认值。任务创建必须从项目上下文继承 `project_id`，不在任务表单里再次选择或更换项目。项目工作区不再显示“全部项目”筛选器；面包屑支持从任务详情或创建页返回当前项目，再回到项目列表。新建项目后直接进入其工作区，空项目显示“创建第一个任务”。

项目内任务列表始终请求 `GET /pipelines?uid=...&project_id=当前项目ID`，详情返回的 `project_id` 必须与当前项目上下文一致；不能通过只在前端过滤全部任务来代替项目范围查询。

修改项目默认教师只影响新任务的默认选择，已有任务使用各自配置快照。复用配置仍在原项目中创建新任务。全局数据集页点击“用于蒸馏”时，先选择所属项目，再进入创建流程，并保留已选数据集；数据不会因此迁移到项目目录或改变归属。

任务详情为“运行与产物 / 样本与质量 / 日志”。首屏先展示执行进度，再并列展示数据和学生模型；配置及存储版本折叠。UUID 不作为列表主标题。训练失败且数据已导出时，数据预览、下载和去训练仍可用。

| 创建步骤 | 配置内容 | 行为 |
|---|---|---|
| 目标与学生 | 两种产出目标、任务名、五类数据目标、配方、学生基模 | 项目由父级上下文固定，面包屑展示；学生基模始终可见，只生成数据时禁用，开启训练后必选；视觉任务禁选不支持视觉的基模 |
| 数据与教师 | 独立数据集、首条预览、教师服务/API | “质量与生成设置”“字段映射”默认折叠；切换配方显示对应设置 |
| 资源与确认 | 单个训练节点、具体 GPU 卡号、简明配置摘要 | 训练参数、CPU 数据节点和运行环境折叠；未选卡不能创建；创建为 pending，再进入详情启动 |

SFT 用质量阈值；CoT 用逻辑正确性、RV/CD 和难度分组；DPO 用候选数、评分差、优选/差选；Agent 用步骤数、轨迹数和工具模式；多模态须检查图文输入和教师能力。

第一步以两张产出选择卡表达“只生成数据”和“数据 + 学生模型”，默认前者；学生基模选择器始终可见，开启训练后必须从当前用户的本地基模目录选择。第三步集中处理训练节点、卡号与提交确认，避免另开冗长的配置汇总页面。LoRA/全量、轮数、学习率、每卡批量和对话模板放在“训练参数”内；CPU 数据节点及离线运行环境放在另一个折叠项。前后切换步骤保留已填内容。

页面不展示请求 JSON、服务器路径输入框或整段业务说明。底部用简短产出流程提示“种子数据 → 数据集 → 学生模型”。样本默认一次查看一条，DPO 并排显示优选与差选；评分筛选收进折叠项。列表使用舒适密度，宿主设计控件可切换为紧凑密度。

### 节点与 GPU 交互补充

- 节点与卡号放在项目内创建流程的第三步。新版默认打开项目列表，进入项目后创建任务；通过项目面包屑或“返回项目”回到项目工作区。顶栏统一标记“示例环境”；节点、显存、占用和服务在线状态均为合成示例，不代表实时库存。
- 数据执行节点与学生训练节点分开。数据节点可省略，使用后端配置的默认节点；数据阶段只申请 CPU，教师服务所用卡由原推理服务管理。
- 训练节点为单选，卡号为复选。型号和每卡显存集中展示一次；卡号矩阵只展示卡号及勾选状态，占用/不可调度卡禁选，另有简短图例。正式前端的实时占用信息须来自平台监控，不能把 `available` 或 `enable_xpu_indices` 当作空闲证明。
- 只有选择训练节点后才出现该节点的卡列表，至少选一张。切换节点清空之前的卡号；在同一节点前后切换创建步骤保留选择。卡数由所选数组长度计算，不提供独立卡数输入框；校验信息显示在对应选择区域。
- `selected_resources` 仅提交一个节点及 `xpu_indices`；可同步提交与数组长度一致的 `gpu_count`。`executor_nodes` 仅用于 CPU 数据节点。关闭学生训练后不提交训练节点和 GPU 字段，即使界面保留草稿选择。
- 第三步实时显示卡数和单卡/单机 DDP，数据节点可在运行环境中查看。任务详情的“任务配置”保留节点与卡号快照。仅在学生训练阶段申请 GPU，不承诺从任务创建时开始预留。
- 多卡 DDP 每卡保留一份模型，不把显存相加显示为“模型可用显存”；没有跨节点、ZeRO/FSDP 或自动显存容量推荐控件。
- 此处选择的是服务器节点和卡，不提供任意跳过流水线阶段的功能。外层四阶段由配方确定，开启训练后增加第五个 `student_train` 阶段。

例如选择同一节点的 6、7 号卡：

```json
{
  "resource_request": {
    "gpu_count": 2,
    "selected_resources": [
      {"node_name": "specialai-3090-0", "xpu_indices": [6, 7]}
    ]
  }
}
```

学生列表来自 `GET /api/v1/models/student?uid={uid}`。模型项保存 ID，名称用于展示；提交到 `training_config.distillation.student_model.model_id`，后端解析当前用户目录内路径。学生直接加载本地权重进行训练，无需部署推理服务。列表为空时显示“暂无可用学生基模，请先将模型纳入当前用户模型目录”，加载失败可重试，不能退化为要求填写服务器路径。Mock 的模型为明确标注的示例，不能当作真实库存。

本次教师配置保存为 `training_config.distillation.teacher_model` 快照；省略时从项目默认教师取得。学生使用 `student_model.model_id`，后端从当前用户模型目录解析路径。教师或学生的每次选择不能通过修改项目默认值来间接改变其他任务。训练请求还必须提交正数 `num_train_epochs`、`learning_rate`；当前选择单个训练节点，可选择该节点的多张 GPU。

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
| 项目、用户与数据集的关系 | 项目入口、上下文继承和逐层返回已联动 | 任务必须归属项目，数据属于用户；项目创建、设置和真实统计仍需接现有 API |
| 教师推理服务与兼容 API | 已表达选择入口 | 模型中心解析 worker 可达的连接信息；示例请求没有完整演示密钥和服务发现 |
| 核心数据目标 | 指令、CoT、DPO、Agent、多模态五类 | 当前可选 11 个配方，后端目录为 15 个；独立 instruct_eval/cot_eval、search_agent_distill、mm_cot_distill 未在 Mock 单独展开 |
| 输入数据与存储 | 已表达独立数据集、来源与目录 | 上传在本地解析 JSON/JSONL，候选登记使用合成记录；正式登记、字段映射与图片附件检查需接真实接口 |
| 数据质量 | SFT 阈值和 DPO 评分差可交互 | DPO 按 chosen/rejected 分差筛选，不能按单个回答的有用性分筛选；其他配方参数按原生 YAML 实现 |
| 学生训练 | 基模、镜像、模板、微调方式、参数、GPU | 首步选基模，末步选节点/卡；参数保存为示例草稿，复用配置保留选择。正式前端须替换示例资源数据，按实际合同组装请求 |
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
- 仅接受 EasyDistill 2 的 training_config.distillation；旧版任务和兼容字段已清理，不再返回 summary.retired、trigger_mode 或 retry_count。stage.node_name 为实际执行节点：控制阶段取控制服务主机名，容器阶段取调度器确认的 worker；尚未分配时为空。
- JSON 数组按对象计数；JSONL 严格逐条解析，非法行不静默丢弃。TXT 不作为 2.0 可直接运行的格式推荐。
- 不丢失 messages、images、工具轨迹、偏好对及 metadata，不能一律压成 instruction/output 两列。
- 无产物不能显示成功；数据生成失败不叫“模型训练失败”。
- 预览最多 100 条，不能据此推算全量评分分布。`/quality` 提供按中间产物全量扫描的评分统计；token 和成本返回 null，禁止填演示数字。
- Agent 默认模拟工具，不标注为已接真实业务系统。Agent、图文指令和图文 CoT 已完成真实教师到学生 LoRA 的小样本验收；业务质量仍需单独评估。
- 图像/视频生成等尚未安装额外依赖的能力不能直接开放为“已可运行”。
- 当前保留中间及恢复文件，没有通用断点续跑 API，不提供无效续跑按钮。
- 目录内 15 个配方经过配置编译检查。基础指令、CoT、质量筛选和 DPO 进行镜像及实际 GCS 验收，其余需逐项做模型效果验收。
- LLaMA-Factory 单卡及双卡 DDP 的 SFT/DPO LoRA 功能已验收；8 卡训练、910B 适配及学生模型效果尚待验证。

`ui/gcs-distill2-mock.html` 可直接打开；`ui/gcs-distill2-mock.fragment.html` 是唯一可编辑源，修改后应重新导出，避免嵌套包装。默认打开项目列表，支持项目工作区、层级导航、固定项目归属的三步创建、节点/卡号、草稿保留、任务启动/取消状态示例、复用配置、DPO 比较、筛选、数据预览，以及项目创建和设置。上传文件只在当前页面解析；数据下载提示对应文件入口，不生成假生产数据。权重只演示文件入口，不生成假权重；跨模块训练只提示跳转动作。Mock 不连接生产 API，也不提供完整原生参数编译器；正式请求必须读取 `/workflows` 的默认配置及 OpenAPI。前端应复用现有组件实现，不替换整个平台导航。

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

路径省略统一前缀 `/api/v1`。状态为 pending → scheduled → running → succeeded/failed/canceled；不要把中间算子日志当成外层阶段。缺少本地镜像会报错，所有蒸馏容器均禁止联网拉取镜像或模型。教师服务通过内网 HTTP 调用，学生权重必须预先完整放入当前用户模型目录。

### 复用及跳转训练

`POST /pipelines/{id}/clone?uid=...` 从服务端配置快照复制为新的 pending 任务，不启动。先将详情中的脱敏配置放入创建表单，用户编辑后调用 clone；可选 body 包含 `name`、`dataset_id`、`training_config`、`resource_request`。training_config 对象递归合并，数组整体替换；resource_request 整体替换。未提交的配置保留原值；UID 和 project_id 固定继承原任务，不接受覆盖。教师连接未改变时，省略密钥或传原掩码都保留服务端密钥；更换教师连接须重新输入密钥。副本重新校验教师、数据和学生模型，不承诺复用已下线服务，原任务不修改。

“去训练 / 使用此数据集训练”跳转现有模型训练页面，并携带 distill 数据集 ID。训练页面调用 `GET /datasets/{id}/training-config?uid=...`，获取 SFT/DPO stage 和 LLaMA-Factory dataset_info，加入现有训练创建流程。这个接口不创建训练任务、不复制数据、不注册模型。源数据仍在当前用户共享目录，训练模块挂载用户目录并复用已有调度。scored_records 和普通 records 返回 422，不能误作可训练数据。

资源接口中的 available/enable_xpu_indices 表示平台允许调度。新版 `/resources/available` 已接入 worker → Redis → gcs-v2 的现有监控链路，同时返回真实显存、利用率、进程数和卡级 selectable；前端直接使用下述合同。

服务重启会恢复尚未执行的 scheduled 任务；已经运行且中断的任务标记失败、回收容器，保留成功的数据资产。没有断点续跑。

### 原生默认参数与质量图表

`GET /workflows` 每项返回 `default_parameters`。切换配方时用它初始化表单，再修改对应生成、评分、过滤参数，提交到 `training_config.distillation.parameters`。数组会整体替换：编辑阈值时必须保留完整 pipeline 算子数组，不能只提交一个过滤算子。后端控制教师和路径，不提供它们作为可编辑默认值。

`GET /pipelines/{id}/quality` 每个指标提供 histogram 和 histogram_bin_width=0.1，可绘制全量评分分布；桶标签为区间下界，other 表示超出桶数上限的长尾。count/min/max/mean 为精确聚合，不把直方图区间当作精确任意阈值过滤结果。DPO 的 candidate_score、chosen_score、rejected_score、preference_margin 分开显示。每个中间产物独立汇总，不能把同一批样本的多个产物累计当作总样本数。

“全部”筛选不发送对应 status/job_type/source_type 值，不传字面字符串 all。

## 固定页面接口细节（单机多卡）

此次不修改 mock 布局。下面是正式前端逐项对接依据，统一省略 `/api/v1` 前缀。

| 页面位置/控件 | 读取和写入合同 |
|---|---|
| 项目首页、创建、设置 | `/projects` GET/POST、`/projects/{id}` GET/PUT/DELETE；名称去首尾空格，默认教师可空；统计读 `/{id}/summary` |
| 项目工作区、任务搜索 | `/pipelines?uid=...&project_id=...`；name/status/job_type 可选；详情及所有子接口可附 project_id，错误项目上下文返回 404 |
| 两种产出、配方 | `/workflows` 读取 job_type、input_fields、output_format、default_parameters；选择写 train_student、job_type；records/scored_records 禁止接学生训练 |
| 学生、训练参数、运行环境 | `/models/student`、`/runtime`；train_student=true 才校验基模和训练参数；trainer_image 可省略沿用配置镜像，指定值必须在 runtime 目录中；模板省略时尝试使用基模 suggested_template |
| 数据选择、上传、候选登记 | `/datasets` GET/POST、`/datasets/upload` POST、`/datasets/candidates` GET；上传及候选文件实际计数，空文件/非法 JSON 返回 400；不能信任客户端 record_count |
| 字段映射、预览 | `/datasets/{id}/preview`；field_mapping 键为规范字段、值为源列名；原始 metadata、messages、images 不丢弃 |
| 教师选择与测试 | `/teachers`、`/teachers/test`；supports_vision 可为 null，capability_source=model_config/unknown；配置识别不等于已做视觉推理验收；连接测试仍返回 vision_verified=false |
| 输出长度、并发 | teacher_model.max_tokens、concurrency 为正数时覆盖配方生成配置和已有阶段对应设置；省略/0 保留配方默认，负数拒绝；timeout_seconds 控制教师请求超时 |
| CPU 数据节点 | `/runtime.data_workers` 与 `/resources/available.nodes[].data_supported`；executor_nodes 最多一个，可省略沿用默认；不申请 GPU |
| 训练节点及卡号 | `/resources/available`；单节点 radio + devices 复选；POST `/resources/validate` 做提交前实时校验，不预留 GPU |
| 确认、创建、启动 | `/workflows/preview` 返回脱敏执行配置、四/五阶段、归一化 resource_request；POST `/pipelines` 创建 pending；POST `/{id}/start` 单独启动 |
| 复用、取消 | POST `/{id}/clone` 提交编辑结果；POST `/{id}/cancel` 后以查询状态为准；无原地重试/续跑 API |
| 运行、样本、日志 | `/{id}/stages`、`/artifacts`、`/artifacts/preview`、`/quality`、阶段 logs/httpstream/ws/download；读取真实状态，数据和模型独立判断可用性 |
| 下载、去训练 | `/datasets/{id}/download`、任务 `/artifacts/download`、`/model/download`、数据集 `/training-config`；保留已有共享存储和训练模块接口 |

资源 `devices[]` 包含 index、name、uuid、memory_total_mib/memory_used_mib/memory_free_mib、utilization_percent、process_count、hardware_enabled、software_enabled、scheduler_available、selectable、state。显存单位 MiB，展示 GiB 时除以 1024；节点 node_memory 单位 GiB。未知测量值为 null，不能显示为 0。observed_at 为采集时间，超过两分钟禁止选择；没有 NVML 进程测量也禁止选择。

| state | 展示与操作 |
|---|---|
| available | 空闲，可勾选，唯一 selectable=true 的状态 |
| in_use | 有进程或利用率大于 0，显示占用并禁选 |
| reserved | 平台已分配或软件禁用，禁选 |
| telemetry_unknown / telemetry_stale | 监控未知/过期，显示重试入口并禁选 |
| node_unavailable / hardware_unavailable / unsupported_runtime | 节点不可用/卡异常/当前 x86 NVIDIA 运行时不支持，禁选 |

提交前调用 `POST /resources/validate`，body 直接为 resource_request，成功返回 `valid=true`、`reserves_resources=false` 和归一化资源。空卡号、重复/负数、卡数不符、多节点返回 400；节点/卡当前不可用返回 409。只提交一个 selected_resources，gpu_count 可省略，由卡号数组长度推导。创建 pending 允许保留当前繁忙卡的选择，但页面正常仅允许勾选 selectable 卡；开始任务及真正进入 student_train 前再次检查。数据阶段不预占 GPU，实际资源仍由 GCS 原有分配器最终分配。实际分配失败时保留已导出的数据和阶段错误。

### 配方控件写入位置

所有配方设置写入 `training_config.distillation.parameters`，以 `/workflows.default_parameters` 初始化后编辑完整对象。以下为具体位置；pipeline 数组必须保留其余算子及顺序。

| 配方设置 | 原生配置位置 |
|---|---|
| 高质量 SFT 的各项最低分、保留比例 | pipeline 中 quality_filter.config.min_scores、keep_top_ratio |
| CoT 逻辑正确性、难度分组、RV 策略 | advanced_cot_distill 的 cot_mix_by_rv_cd.config.min_correctness、cd_bins、rv_target；rv_target 用 matched/low/medium/high 或数值，不能把它描述成精确 token 长度 |
| DPO 候选数、评分差 | generate_candidates.config.n、build_preference_pairs.config.min_margin |
| Agent 步骤/轨迹 | agent_trajectory.config.max_steps、repeat_times；保留配方工具模式，默认模拟工具；不声称接入真实业务系统 |
| 多模态 | images 数组非空、视觉教师/学生能力检查；附件必须对运行节点可读，不能把浏览器本地文件名当成已上传附件 |

### Agent 与图文执行补充

- Agent 的全局 generation.system_prompt 会应用到任务生成、任务改写、工具检查和 rubric 等提示阶段，并补充 XML/JSON 数组输出约束。保留各阶段明确指定的 system_prompt。无法解析的教师输出仍由原生算子过滤，最终无有效数据则任务失败，不制造样本。
- Agent 和图文训练未指定 `training_config.max_length` 时保存为 4096；明确传入的值保持不变，避免沿用短文本默认长度而截掉工具提示或图片后的监督内容。Mock 布局无需变动。
- 图文输入 images 使用当前用户 JFS 图片路径或 data:image 内嵌图片。HTTP 外链在任务创建校验时拒绝；前端文件选择后应使用平台文件机制对应的共享路径，不能提交临时浏览器 URL。
- 原生图文输出保留在 `data/intermediate/mm_native_sft.jsonl`；正式导出将 messages 的图文数组转换为带 `<image>` 的文本，images 为有序附件路径。附件写入该用户 `datasets/generated/{dataset_id}/images/`，不依赖任务目录；元数据保留。预览按图片标记顺序渲染，图片展示继续走现有用户文件访问与鉴权。
- 学生视觉基模 `Qwen2-VL-2B-Instruct` 已完整入库、实际训练验收；模板为 qwen2_vl。运行镜像从 `/runtime.trainer_images` 读取，不在前端写死旧 tag。

后端生成阶段执行配置会固化这些值。新增契约验收脚本 `tests/integration/frontend_contract_smoke.py` 覆盖页面 11 个配方预览、真实资源、错误选择、参数编辑复用、项目/用户隔离；已有 control/student 脚本覆盖实际数据生产及双卡训练。

## 用户存储隔离更新

学生列表和详情必须传 uid；同一 model_id 在不同用户下独立解析，禁止用前一用户的列表缓存。切换账号时清空学生选择及模型缓存。`/runtime.weights_source` 为 `user_private_model_pool`。目录与离线导入命令见 `storage-layout.md`。其余页面布局及任务字段保持不变。
