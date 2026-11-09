# 产物记录数与样本评分对接

适用控制端 v2.0.10，API 前缀 `/modelDistillIpAddress/api/v1`。UID 从登录态获取。后端更新不会自动更新平台前端。

## 1. 全部产物文件

调用 `GET /pipelines/{id}/artifacts?uid=338`，读取 `data.items`：

```json
{
  "name": "dataset.jsonl",
  "relative_path": "data/exported/dataset.jsonl",
  "kind": "dataset",
  "bytes": 20361,
  "previewable": true,
  "record_count": 10,
  "record_count_status": "counted"
}
```

| record_count_status | record_count | 页面显示 |
|---|---|---|
| counted | 整数，包含 0 | 实际的 N 条；只有此时 0 才代表空文件 |
| not_applicable | null | — |
| unavailable | null | 暂无法统计，可刷新或查看文件 |

统计遍历完整 JSON/JSONL，不使用预览长度或字节数推算。文件正在变化、格式不完整或不可读取时不返回假 0。

`kind=dataset_metadata` 是 manifest.json、expansion.json 等说明文件，不能标为“最终训练数据”；模型权重和模型元数据也没有样本条数。建议用途按以下顺序判断：

1. dataset_metadata：产物清单、扩充统计或划分说明。
2. model_weights / model_metadata：模型权重 / 模型配置。
3. data/exported/dataset.jsonl：最终全量数据。
4. data/exported/splits/ 下 train/validation/test.jsonl：训练集 / 验证集 / 测试集。
5. data/intermediate/ 下 dataset：阶段中间数据，不能统一标为最终训练集。

前端不要写 `row.record_count || 0`，也不要把 null 强制转为 Number：

```js
function recordCountText(row) {
  if (row.record_count_status === 'counted' && Number.isInteger(row.record_count)) {
    return `${row.record_count} 条`;
  }
  return row.record_count_status === 'unavailable' ? '暂无法统计' : '—';
}
```

## 2. 样本预览与评分

调用 `GET /pipelines/{id}/artifacts/preview?uid=338&path=data/exported/dataset.jsonl&limit=10`。原 `data.items/total/fields/limit` 保留，增加 `data.quality`：

```json
{
  "scope": "preview_records",
  "scored_records": 10,
  "unscored_records": 0,
  "ambiguous_records": 0,
  "items": [
    {
      "status": "scored",
      "metrics": {"informativeness": 8, "helpfulness": 8, "generalization": 8, "correctness": true},
      "sources": ["data/intermediate/expansion-01/04_instruct_eval.jsonl"]
    }
  ]
}
```

上例截取一个评分对象；实际 `quality.items.length` 与 `data.items.length` 完全相等，按相同索引对应。换文件或刷新后整体替换，不能沿用上一个文件的评分。

| status | 页面行为 |
|---|---|
| scored | 展示 metrics 中实际存在的各维度评分 |
| unavailable | 显示“未记录评分”，不显示 0 分 |
| ambiguous | 显示“存在多份评分，无法唯一关联”，可展开 sources 查看中间文件 |

数值 0 是有效评分，correctness=false 是“不通过”，不能用真假值判断是否缺失。没有的维度显示“—”。后端没有提供统一综合分，不能随意把正确性布尔值和 0–10 维度相加平均后标为综合质量。

“最低有用性评分”滑块只对 `metrics.helpfulness` 为数值的已加载样本起作用。缺少该字段的样本单独提示，默认保留显示，可另加“仅看有评分样本”开关；这只是本次预览过滤，不修改原文件和任务质量规则。

后端对已有 SFT 产物，在当前任务中按问题和回答的完整文本关联评分，不按跨轮重复的 request_id 或数组序号关联。匹配不唯一且评分冲突时标为 ambiguous；找不到或只有图文/多轮/工具轨迹且未存评分时标为 unavailable。不会重新调用教师、生成评分或回写历史文件。

新任务在数据导出时把实际找到的评分保存到 `metadata.quality`，之后生成的可复用数据资产和训练/验证/测试快照都会携带评分。训练配置依旧只读取原来的 messages/images 或 DPO 字段。已有任务的预览补充放在响应的 quality 中，下载历史文件仍是原始文件，SHA256 和已有划分快照不改变。

真实空中间文件现在预览返回 200、items=[]、total=0；种子上传校验和最终导出仍拒绝空数据。manifest.json / expansion.json 的预览仍返回原始说明 JSON，没有样本 quality 结构，前端应按文件 kind 分开渲染。

## 3. 整个数据集的评分统计

调用 `GET /pipelines/{id}/quality?uid=338`：

- `data.exported_dataset`：最终全量数据的 records、scored_records、unscored_records、ambiguous_records 和 metrics 统计；尚无可读最终文件时为 null。
- `data.scored_artifacts`：各轮评分、质量过滤等中间文件的独立统计，包含 relative_path 和上述计数。不能把各轮数字相加当作最终数据量。
- `data.unavailable_artifacts`：暂时无法解析/统计的文件路径，应提示后刷新，不能当作 0 分。
- 每项 metrics 的 count/min/max/mean/histogram 仅计算实际存在的评分；布尔值统计中 true=1、false=0，页面仍应按正确率等语义显示。
- token_usage/cost 仍为 null；model_evaluation=not_performed 是接口的范围说明，不渲染为学生模型的状态标签。当前蒸馏模块未实现学生独立效果评测，删除“效果尚未测评/独立效果未评测”等标签和评测提示。数据评分仍正常展示。

顶部“全部样本”统计使用 exported_dataset；预览区提示使用 quality.scope=preview_records 的计数。两者不能混用。接口地址、身份与任务归属校验保持一致。

## 4. 截图任务联调值

UID 338、任务 `f1611f3b-ffe3-4d3c-8948-e420b6351914`：最终 10 条、train 8 条、validation 1 条、test 1 条。其记录数来自实际文件，评分来自已有中间评分记录。具体评分覆盖情况以线上接口返回和验收记录为准。

## 5. 学生模型卡片去掉未实现的效果评测标签

当前模块没有独立的学生模型效果评测，不把“尚未评测”作为任务状态展示：

- 删除学生模型标题旁的“效果尚未测评”标签，不改成另一个“未评测”标签。
- 完成提示只保留“数据生成与学生训练均已完成”，删除后面的未评测说明及独立评测模块引导。
- 保留“LoRA 权重已产出”、基模说明、下载权重和训练日志。未完成时按真实 student_train 状态展示等待、训练中或失败。
- train_loss / eval_loss 若存在可在训练指标中显示；缺失时不填 0，也不作为综合效果分。
- 不根据 model_evaluation 字段生成标签。这个字段没有实现独立效果评测能力，也不新增效果评测按钮。

这部分是前端展示调整，不需要改训练参数、重跑任务或重启后端。仓库 mock 已同步删除标签；正式平台前端需要在自己的任务详情组件中实施上述修改。
