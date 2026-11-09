# 产物统计与样本评分验收

2026-11-09，控制端 v2.0.10-x86-offline，部署于 172.18.29.80。平台 172.18.127.67 的 modelDistillIpAddress 代理已验证返回新字段。

## 自动检查

`go test ./...`、`go vet ./...`、OpenAPI 格式校验通过。覆盖：完整文件计数、空文件与空 JSON 数组、损坏文件不发布局部统计、说明文件条数为 null、种子仍拒绝空数据、预览长度不影响总条数、0/false 评分保留、重复 ID 不误关联、同问题不同回答不混用、冲突评分不任意选择、损坏中间文件不使用部分评分、历史文件只读、新导出资产与划分快照保存评分、接口跨用户 404。

## 已有真实任务复核

UID 338，任务 `f1611f3b-ffe3-4d3c-8948-e420b6351914`，状态 succeeded。没有重新生成样本或调用教师，没有新增业务验收项目/任务/数据集。

| 文件 | 实际记录数 |
|---|---:|
| data/exported/dataset.jsonl | 10 |
| train.jsonl | 8 |
| validation.jsonl | 1 |
| test.jsonl | 1 |
| expansion-03/05_quality_filter.jsonl | 0 |

产物共 54 个，说明文件与模型文件返回 null / not_applicable，真实空中间文件返回 0 / counted，空文件预览返回 200。最终 10 条及 8/1/1 分区预览都关联到真实评分，最终统计 scored_records=10、unscored_records=0、ambiguous_records=0。

最终保留样本的 helpfulness 平均 7.9、informativeness 平均 8、generalization 平均 7、correctness=true 为 10/10。它们是已有教师记录的数据评分，不代表学生模型效果。

本次单次接口耗时：产物列表约 0.052 秒，最终样本预览约 0.039 秒，质量统计约 0.051 秒。这是当前小任务实测，不是大数据量性能承诺。

操作前后比较该任务 data 下 37 个文件的 SHA256，全部一致，历史导出和划分快照未改写。更新时 active_tasks=0；数据库计数前后均为 datasets=27、pipelines=40、projects=7。仅替换并重启蒸馏控制端，配置、共享教师、GCS、数据/训练镜像保持原配置，没有新依赖和数据库迁移。

二进制 SHA256：`6cd84907a0326327a9585cd3ac5b99d2830be862964f6d486e946633fa48f6f9`。

## 验收边界

平台前端尚需按 artifact-quality-frontend.md 绑定新字段；本次没有修改平台前端。新导出评分持久化由本地导出/资产/划分集成测试覆盖，没有为此重复启动生产教师生成和学生训练。旧产物预览通过精确文本关联只读补充评分，下载仍是原始文件。未存评分的图文、多轮、工具轨迹不做猜测匹配；找不到记录显示 unavailable，有冲突显示 ambiguous。
