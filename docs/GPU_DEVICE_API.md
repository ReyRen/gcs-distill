# GPU 设备指定功能 - 前端接口文档

## 概述

GPU 设备指定功能现已完全集成到前端 API 接口中。前端可以通过 `resource_request.gpu_device_ids` 字段指定使用具体的 GPU 设备。

## API 接口

### 创建流水线时指定 GPU

**POST** `/api/v1/pipelines`

```json
{
  "project_id": "uuid-xxx",
  "dataset_id": "uuid-yyy",
  "training_config": {
    "num_train_epochs": 3,
    "learning_rate": 0.0001
  },
  "resource_request": {
    "gpu_count": 2,
    "gpu_device_ids": "0,1",
    "memory_gb": 32,
    "cpu_cores": 8
  }
}
```

### 字段说明

**resource_request.gpu_device_ids** (可选字符串)
- 格式：逗号分隔的 GPU 设备 ID，如 `"0,1,2"`
- 示例：
  - `"0"` - 仅使用 GPU 0
  - `"0,1"` - 使用 GPU 0 和 1
  - `"1,3,5"` - 使用 GPU 1、3、5
- 留空则由系统根据 `gpu_count` 自动分配

**优先级规则**：
- 如果同时指定 `gpu_count` 和 `gpu_device_ids`，优先使用 `gpu_device_ids`
- 如果只指定 `gpu_count`，系统自动分配相应数量的 GPU
- 如果 `gpu_count` 设置为 `-1`，使用所有可用 GPU

## 前端实现示例

### TypeScript 类型定义

```typescript
interface ResourceRequest {
  gpu_count: number;
  gpu_device_ids?: string;   // 新增字段
  gpu_type?: string;
  memory_gb?: number;
  cpu_cores?: number;
}

interface PipelineRun {
  id: string;
  project_id: string;
  dataset_id: string;
  training_config: TrainingConfig;
  resource_request: ResourceRequest;
  status: PipelineStatus;
}
```

### React 表单示例 (Ant Design)

```jsx
import { Form, InputNumber, Input, Tooltip } from 'antd';
import { QuestionCircleOutlined } from '@ant-design/icons';

function PipelineForm() {
  return (
    <Form layout="vertical">
      {/* GPU 数量 */}
      <Form.Item
        label="GPU 数量"
        name={['resource_request', 'gpu_count']}
        rules={[{ required: true, message: '请输入 GPU 数量' }]}
      >
        <InputNumber min={1} max={8} placeholder="1" />
      </Form.Item>

      {/* GPU 设备 ID - 新增 */}
      <Form.Item
        label={
          <span>
            GPU 设备 ID&nbsp;
            <Tooltip title="指定使用的 GPU 设备，如 '0,1' 表示使用 GPU 0 和 1。留空则自动分配">
              <QuestionCircleOutlined />
            </Tooltip>
          </span>
        }
        name={['resource_request', 'gpu_device_ids']}
      >
        <Input placeholder="0,1,2" />
      </Form.Item>

      {/* 内存 */}
      <Form.Item
        label="内存 (GB)"
        name={['resource_request', 'memory_gb']}
      >
        <InputNumber min={8} max={512} placeholder="32" />
      </Form.Item>

      {/* CPU */}
      <Form.Item
        label="CPU 核心数"
        name={['resource_request', 'cpu_cores']}
      >
        <InputNumber min={1} max={64} placeholder="8" />
      </Form.Item>
    </Form>
  );
}
```

### Vue 表单示例 (Element UI)

```vue
<template>
  <el-form :model="pipelineForm" label-width="120px">
    <!-- GPU 数量 -->
    <el-form-item label="GPU 数量">
      <el-input-number
        v-model="pipelineForm.resource_request.gpu_count"
        :min="1"
        :max="8"
      />
    </el-form-item>

    <!-- GPU 设备 ID - 新增 -->
    <el-form-item label="GPU 设备 ID">
      <el-input
        v-model="pipelineForm.resource_request.gpu_device_ids"
        placeholder="0,1,2"
      >
        <template #append>
          <el-tooltip content="指定使用的 GPU 设备，如 '0,1' 表示使用 GPU 0 和 1">
            <i class="el-icon-question"></i>
          </el-tooltip>
        </template>
      </el-input>
    </el-form-item>

    <!-- 内存 -->
    <el-form-item label="内存 (GB)">
      <el-input-number
        v-model="pipelineForm.resource_request.memory_gb"
        :min="8"
        :max="512"
      />
    </el-form-item>
  </el-form>
</template>

<script>
export default {
  data() {
    return {
      pipelineForm: {
        resource_request: {
          gpu_count: 1,
          gpu_device_ids: '',  // 新增字段
          memory_gb: 32,
          cpu_cores: 8
        }
      }
    };
  }
};
</script>
```

## 使用场景

### 场景 1: 自动分配 GPU

用户只关心 GPU 数量，不关心具体使用哪个设备：

```json
{
  "resource_request": {
    "gpu_count": 2
  }
}
```

系统会自动分配 2 个可用的 GPU。

### 场景 2: 指定特定 GPU

用户知道服务器上哪些 GPU 性能更好或更空闲：

```json
{
  "resource_request": {
    "gpu_count": 2,
    "gpu_device_ids": "0,1"
  }
}
```

系统会使用 GPU 0 和 1（`gpu_device_ids` 优先级更高）。

### 场景 3: 使用所有 GPU

用于大规模训练任务：

```json
{
  "resource_request": {
    "gpu_count": -1
  }
}
```

系统会使用节点上所有可用的 GPU。

### 场景 4: 避开繁忙的 GPU

如果 GPU 2 正在被其他任务使用：

```json
{
  "resource_request": {
    "gpu_count": 2,
    "gpu_device_ids": "0,3"
  }
}
```

明确指定使用 GPU 0 和 3，避开 GPU 2。

## 表单验证建议

```typescript
// GPU 设备 ID 验证规则
const gpuDeviceIdsValidator = (rule: any, value: string) => {
  if (!value) return Promise.resolve(); // 可选字段

  // 格式验证：逗号分隔的数字
  if (!/^\\d+(,\\d+)*$/.test(value)) {
    return Promise.reject('格式错误，应为逗号分隔的数字，如 0,1,2');
  }

  // 检查重复
  const ids = value.split(',');
  if (new Set(ids).size !== ids.length) {
    return Promise.reject('设备 ID 不能重复');
  }

  // 检查范围（假设最多 8 个 GPU）
  if (ids.some(id => parseInt(id) > 7)) {
    return Promise.reject('设备 ID 超出范围（0-7）');
  }

  return Promise.resolve();
};
```

## 完整示例

```jsx
import React, { useState } from 'react';
import { Form, Input, InputNumber, Button, message } from 'antd';
import axios from 'axios';

function CreatePipelineForm({ projectId, datasetId }) {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (values) => {
    setLoading(true);
    try {
      const response = await axios.post('/api/v1/pipelines', {
        project_id: projectId,
        dataset_id: datasetId,
        training_config: values.training_config,
        resource_request: values.resource_request
      });

      message.success('流水线创建成功');
      console.log('Pipeline ID:', response.data.data.id);
    } catch (error) {
      message.error('创建失败: ' + error.response?.data?.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Form
      form={form}
      layout="vertical"
      onFinish={handleSubmit}
      initialValues={{
        training_config: {
          num_train_epochs: 3,
          learning_rate: 0.0001
        },
        resource_request: {
          gpu_count: 1,
          gpu_device_ids: '',
          memory_gb: 32,
          cpu_cores: 8
        }
      }}
    >
      {/* 训练配置 */}
      <Form.Item label="训练轮数" name={['training_config', 'num_train_epochs']}>
        <InputNumber min={1} max={100} />
      </Form.Item>

      <Form.Item label="学习率" name={['training_config', 'learning_rate']}>
        <InputNumber min={0} max={1} step={0.00001} />
      </Form.Item>

      {/* 资源配置 */}
      <Form.Item
        label="GPU 数量"
        name={['resource_request', 'gpu_count']}
        rules={[{ required: true }]}
      >
        <InputNumber min={1} max={8} />
      </Form.Item>

      <Form.Item
        label="GPU 设备 ID"
        name={['resource_request', 'gpu_device_ids']}
        tooltip="可选，指定使用的 GPU，如 '0,1'"
        rules={[
          {
            pattern: /^\\d+(,\\d+)*$/,
            message: '格式错误，应为逗号分隔的数字'
          }
        ]}
      >
        <Input placeholder="0,1,2" />
      </Form.Item>

      <Form.Item label="内存 (GB)" name={['resource_request', 'memory_gb']}>
        <InputNumber min={8} max={512} />
      </Form.Item>

      <Form.Item label="CPU 核心数" name={['resource_request', 'cpu_cores']}>
        <InputNumber min={1} max={64} />
      </Form.Item>

      <Form.Item>
        <Button type="primary" htmlType="submit" loading={loading}>
          创建流水线
        </Button>
      </Form.Item>
    </Form>
  );
}

export default CreatePipelineForm;
```

## 注意事项

1. **字段格式**：`gpu_device_ids` 必须是逗号分隔的数字字符串（如 `"0,1,2"`），不是数组
2. **可选字段**：`gpu_device_ids` 是可选的，前端可以不传递此字段
3. **优先级**：如果同时提供 `gpu_count` 和 `gpu_device_ids`，后端会优先使用 `gpu_device_ids`
4. **验证**：建议前端进行格式验证，避免无效的输入
5. **用户体验**：建议添加提示信息，说明如何填写 GPU 设备 ID

## 相关文档

- API 参考文档：`docs/api-reference.md`
- 前端实现指南：`docs/frontend-guide.md`
- 技术实现文档：`docs/easydistill-integration-improvements.md`
