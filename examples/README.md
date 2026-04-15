# 种子数据示例 (Seed Data Examples)

本目录包含用于测试 GCS-Distill 平台的种子数据集示例。所有数据集均采用 **JSONL (JSON Lines)** 格式。

## 文件说明

### 1. seed_data_customer_service.jsonl
**客服问答场景** - 30条样本

适用于训练客服机器人或问答助手，涵盖常见的客户服务场景：
- 账户管理（密码重置、账号注销）
- 订单处理（下单、退款、取消）
- 物流查询（发货、配送）
- 会员服务（积分、优惠券）
- 售后服务（退货、保修）

**使用场景：** 客服机器人、电商问答助手、售后支持系统

### 2. seed_data_ai_ml.jsonl
**AI/机器学习知识问答** - 40条样本

涵盖人工智能和机器学习的核心概念：
- 基础概念（AI、机器学习、深度学习）
- 神经网络架构（CNN、RNN、Transformer）
- 训练技术（优化器、正则化、迁移学习）
- 模型评估（准确率、BLEU、ROUGE）
- 前沿技术（模型蒸馏、LoRA、RAG）

**使用场景：** AI教育助手、技术文档问答、知识库系统

### 3. seed_data_programming.jsonl
**编程教程问答** - 40条样本

覆盖主流编程语言的常见问题：
- **Python**: 基础语法、函数、类、装饰器、异步编程
- **JavaScript**: 变量声明、Promise、async/await、DOM操作
- **Go**: Goroutine、Channel、接口、错误处理

**使用场景：** 编程学习助手、代码问答机器人、技术培训系统

## 数据格式说明

所有种子数据遵循以下 JSONL 格式：

```jsonl
{"instruction": "问题或指令文本", "input": "额外输入上下文（可选）", "output": "预期输出（种子数据通常为空）"}
```

### 字段说明

| 字段 | 是否必需 | 说明 |
|------|---------|------|
| `instruction` | **必需** | 问题、指令或任务描述 |
| `input` | 可选 | 额外的输入上下文或补充信息 |
| `output` | 可选 | 预期输出（种子数据通常为空，由教师模型生成） |

## 快速使用

### 方式一：使用快速测试脚本（最简单）

```bash
# 使用客服问答数据
./examples/quick_test.sh examples/seed_data_customer_service.jsonl

# 使用AI/ML知识数据
./examples/quick_test.sh examples/seed_data_ai_ml.jsonl

# 使用编程教程数据
./examples/quick_test.sh examples/seed_data_programming.jsonl
```

这个脚本会自动完成：创建项目 → 上传数据集 → 启动流水线 → 查看状态

### 方式二：手动步骤

#### 1. 创建项目

```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "客服问答蒸馏",
    "description": "将 Qwen2.5-7B 蒸馏到 Qwen2.5-0.5B",
    "teacher_model": {
      "model_name": "Qwen/Qwen2.5-7B-Instruct",
      "provider_type": "local"
    },
    "student_model": {
      "model_name": "Qwen/Qwen2.5-0.5B-Instruct"
    }
  }'
```

记录返回的 `project_id`。

#### 2. 上传种子数据集

**使用客服问答数据**
```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects/{project_id}/datasets \
  -F "file=@examples/seed_data_customer_service.jsonl"
```

**使用AI/ML知识数据**
```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects/{project_id}/datasets \
  -F "file=@examples/seed_data_ai_ml.jsonl"
```

**使用编程教程数据**
```bash
curl -X POST http://172.18.36.230:18080/api/v1/projects/{project_id}/datasets \
  -F "file=@examples/seed_data_programming.jsonl"
```

记录返回的 `dataset_id`。

#### 3. 启动蒸馏流水线

```bash
curl -X POST http://172.18.36.230:18080/api/v1/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "{project_id}",
    "dataset_id": "{dataset_id}",
    "training_config": {
      "num_train_epochs": 3,
      "learning_rate": 2e-5,
      "per_device_train_batch_size": 4
    }
  }'
```

#### 4. 查看流水线状态

```bash
curl http://172.18.36.230:18080/api/v1/pipelines/{pipeline_id}
```

## 自定义种子数据

### 基本要求

1. **文件格式**: 必须是 `.jsonl` 格式
2. **必需字段**: 每行必须包含 `instruction` 字段
3. **编码格式**: UTF-8
4. **换行符**: 每行一个完整的 JSON 对象

### 示例：创建自定义数据

```bash
cat > my_custom_seed.jsonl << 'EOF'
{"instruction": "如何做番茄炒蛋？", "input": "", "output": ""}
{"instruction": "推荐一些健康的早餐", "input": "", "output": ""}
{"instruction": "夏天适合种什么植物？", "input": "", "output": ""}
EOF
```

### 数据质量建议

1. **多样性**: 涵盖目标领域的不同方面
2. **代表性**: 反映真实用户的典型问题
3. **清晰性**: 问题表述清晰、无歧义
4. **数量**: 建议至少 30-50 条种子数据
5. **长度**: 每条指令建议 10-100 字

## 蒸馏流程

使用种子数据后，系统将自动执行以下流程：

1. **教师模型配置** - 验证教师模型可访问性
2. **蒸馏数据构建** - 加载种子数据
3. **教师推理与样本生成** - 教师模型生成高质量响应
4. **蒸馏数据治理** - 过滤、去重、质量评分
5. **学生模型训练** - 使用生成的数据训练学生模型
6. **蒸馏效果评估** - 生成评估报告

## 常见问题

### Q: 种子数据的 output 字段是否需要填写？

A: 不需要。种子数据的 `output` 字段通常留空，由教师模型在"教师推理与样本生成"阶段自动生成。

### Q: 支持其他格式的数据吗？

A: 目前仅支持 JSONL 格式。如果有 CSV 或 JSON 数组格式，需要转换为 JSONL。

### Q: 种子数据需要多少条？

A: 建议至少 30-50 条。数据量越大，蒸馏效果越好，但也会增加推理成本和时间。

### Q: 可以包含中英文混合的数据吗？

A: 可以。只要 `instruction` 字段有明确的语义即可。

## 数据格式转换

### JSON 数组转 JSONL

```python
import json

# 读取 JSON 数组
with open('data.json', 'r', encoding='utf-8') as f:
    data = json.load(f)

# 写入 JSONL
with open('data.jsonl', 'w', encoding='utf-8') as f:
    for item in data:
        f.write(json.dumps(item, ensure_ascii=False) + '\n')
```

### CSV 转 JSONL

```python
import csv
import json

with open('data.csv', 'r', encoding='utf-8') as csvfile:
    reader = csv.DictReader(csvfile)
    with open('data.jsonl', 'w', encoding='utf-8') as jsonlfile:
        for row in reader:
            jsonlfile.write(json.dumps({
                "instruction": row['question'],
                "input": row.get('context', ''),
                "output": ""
            }, ensure_ascii=False) + '\n')
```

## 参考资料

- [主项目 README](../README.md)
- [API 文档](../docs/api-reference.md)
- [快速启动指南](../docs/quickstart.md)
