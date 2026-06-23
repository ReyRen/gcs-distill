# XPU 资源选择接口

`gcs-distill` 创建流水线时通过 `resource_request` 描述资源需求。资源调度由 `gcs-v2` 统一完成，distill 不直接占用或释放节点资源。

## 创建流水线

```http
POST /api/v1/pipelines
```

```json
{
  "project_id": "project-uuid",
  "dataset_id": "dataset-uuid",
  "training_config": {
    "num_train_epochs": 3,
    "learning_rate": 0.00002,
    "per_device_train_batch_size": 4
  },
  "resource_request": {
    "gpu_count": 1,
    "memory_gb": 32,
    "cpu_cores": 8,
    "selected_resources": [
      {
        "node_name": "gpu-node-01",
        "node_address": "172.18.36.225",
        "xpu_indices": [0]
      }
    ]
  }
}
```

## 字段说明

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `gpu_count` | integer | 需要的 XPU 数量；当前字段名沿用 GPU 命名。 |
| `gpu_type` | string | 可选，资源类型或型号提示。 |
| `memory_gb` | integer | 可选，期望内存。 |
| `cpu_cores` | integer | 可选，期望 CPU 核数。 |
| `selected_resources` | array | 推荐字段，手动指定节点和 XPU 卡。 |
| `gpu_device_ids` | string | 可选，仅表达卡号，例如 `"0,1"`，不包含节点信息。 |

`selected_resources` 的元素结构：

```json
{
  "node_name": "gpu-node-01",
  "node_address": "172.18.36.225",
  "xpu_indices": [0, 1]
}
```

## 推荐策略

- 自动调度：只传 `gpu_count`，由 `gcs-v2` 选择节点和卡。
- 精确绑定：传 `selected_resources`，同时保留 `gpu_count` 表达总需求量。
- 单节点卡号模式：可以传 `gpu_device_ids`，但它无法表达跨节点选择。

自动调度示例：

```json
{
  "resource_request": {
    "gpu_count": 2
  }
}
```

精确绑定示例：

```json
{
  "resource_request": {
    "gpu_count": 2,
    "selected_resources": [
      {
        "node_name": "gpu-node-01",
        "node_address": "172.18.36.225",
        "xpu_indices": [0, 1]
      }
    ]
  }
}
```

## 节点查询

distill 资源选择接口与 `gcs-model-center-v2` 对齐，前端优先使用聚合后的可用资源：

```http
GET /api/v1/resources/available
```

响应中的 `data.nodes` 是前端可直接使用的资源列表：

```json
{
  "nodes": [
    {
      "name": "gpu-node-01",
      "address": "172.18.36.225",
      "available": true,
      "workers_xpuname": "NVIDIA A100",
      "workers_xpucount": 4,
      "enable_xpu_indices": [0, 1],
      "node_cpus": 64,
      "node_memory": 540000000000,
      "node_state": "ready",
      "node_availability": "active",
      "node_role": "worker",
      "node_os": "linux",
      "node_architecture": "x86_64",
      "raw": {
        "brain_worker": {},
        "node": {}
      }
    }
  ]
}
```

原始 `gcs-v2` 节点快照仍然保留，主要用于排障或展示调度侧原始字段：

```http
GET /api/v1/resources/nodes
GET /api/v1/resources/nodes/{name}
```

## 前端校验建议

- `gpu_count` 应为正整数。
- `selected_resources[].node_name`、`node_address` 必填。
- `selected_resources[].xpu_indices` 必须是非空整数数组，且不应重复。
- 若只使用 `gpu_device_ids`，格式应为逗号分隔数字，例如 `0,1,2`。

TypeScript 类型：

```typescript
interface SelectedResource {
  node_name: string;
  node_address: string;
  xpu_indices: number[];
}

interface AvailableNode {
  name: string;
  address: string;
  available: boolean;
  workers_xpuname: string;
  workers_xpucount: number;
  enable_xpu_indices: number[];
  node_cpus: number;
  node_memory: number;
  node_state: string;
  node_availability: string;
  node_role: string;
  node_os: string;
  node_architecture: string;
  raw: unknown;
}

interface AvailableResources {
  nodes: AvailableNode[];
}

interface ResourceRequest {
  gpu_count: number;
  gpu_type?: string;
  memory_gb?: number;
  cpu_cores?: number;
  gpu_device_ids?: string;
  selected_resources?: SelectedResource[];
}
```

更多接口细节以 `/swagger/openapi.json` 和 `/swagger/index.html` 为准。
