# 单轮 SFT 数据复用为指令种子

版本 v2.0.8，2026-11-05。

问题数据集 `d0567d90-5abe-546c-a054-9af1fe0b3ce7` 属于 UID 505，是已有蒸馏生成的两条 SFT 数据，顶层字段为 messages、metadata。旧校验只查 instruction，导致“用于蒸馏”后无法创建指令任务。

基础指令、高质量指令、指令增强、类别均衡四种配方现在接受两种输入：

1. 顶层非空 instruction，或显式 field_mapping 指向的问题文本列。
2. 缺少 instruction 时，使用单轮 messages 中唯一 user 消息的非空文本。

单轮消息顺序为可选 system、一个 user、可选 assistant。只提取 user 问题，任务配置的 system_prompt 继续生效；原 assistant 内容不会充当本次教师的新回答。已有 instruction 和显式字段映射优先，不会因为映射为空而偷偷换用 messages。

转换只发生在校验内存和本次任务 data/seed/input.jsonl 快照，不修改用户原始生成数据。原 messages、metadata 保留。多轮、工具、图文不会自动展平；应整理为对应输入字段或选择合适配方。错误现在包含记录序号，便于定位具体数据。

前端无需为该案例增加 field_mapping；可以用 `/workflows.items[].seed_input_formats` 和 input_note 显示支持 instruction/single_turn_messages，并展示真实种子条数。只含 question 等自定义字段时，仍需 `field_mapping:{"instruction":"question"}`。

用户本次请求选中的是两条生成数据，也可作为目标 500 条的种子。500 是生成后的目标有效数量，与原始种子数量不是一比一关系；教师调用、过滤和预算规则不变。

验证包括单轮自动提取、四种配方、显式映射优先、源文件不变、任务快照标准化、错误记录序号及拒绝多轮/工具/图片输入。生产只调用配置预览验证原请求，不启动 500 条生成，不创建验收项目和数据。

已在平台代理入口复测 UID 505、上述数据集和目标 500 的请求，配置预览返回 HTTP 200。源文件前后 SHA256 均为 `bf64fabbd2ea25444e4f8566375779da94d628a17ec7a44daab7643f83af47b5`；部署前后业务统计均为 7 项目、15 数据集、15 任务，活动任务 0。`go test ./...`、`go vet ./...` 通过。控制端二进制 SHA256 为 `985a189a5bfed90774abd630f1ac35b2d1f4592d545f0394715520438110e9a1`。

本次只更新 gcs-distill 控制端，不变更教师服务、gcs-v2、worker、镜像或数据库表结构。沿用现有离线依赖。
