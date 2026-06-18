# GCS-Distill 集成测试

本目录包含 GCS-Distill 的集成测试脚本，用于验证 EasyDistill 运行镜像和端到端蒸馏工作目录。

## 测试脚本

### `test_easydistill.sh`

验证 EasyDistill Docker 镜像的基础能力：

- 镜像是否存在。
- 镜像命令是否可运行。
- 工作空间卷挂载是否正常。
- 日志文件是否能写入 `/workspace/logs/`。
- 模型输出目录是否可访问。
- GPU 环境可用时，验证 GPU 透传。
- EasyDistill 命令是否可用。

运行方式：

```bash
make docker-build
./tests/integration/test_easydistill.sh
```

### `test_e2e_workflow.sh`

模拟完整蒸馏流程，验证工作空间结构、配置生成、日志和模型输出。

覆盖内容：

- 工作空间目录结构创建。
- 种子数据准备。
- 教师推理和学生训练配置生成。
- 教师推理输出模拟。
- 日志文件持久化和读取。
- 模型 checkpoint 输出。
- Docker 卷挂载。

运行方式：

```bash
./tests/integration/test_e2e_workflow.sh
```

## 快速开始

```bash
make docker-build
make docker-test
./tests/integration/test_easydistill.sh
./tests/integration/test_e2e_workflow.sh
```

## 测试数据

`sample_seed_data.json` 包含 5 条示例种子指令，用于测试蒸馏流程。格式示例：

```json
[
  {
    "instruction": "解释什么是机器学习",
    "input": "",
    "output": ""
  }
]
```

## 故障排查

镜像构建失败：

```bash
docker ps
docker build -t gcs-distill/easydistill:latest -f docker/easydistill/Dockerfile .
```

卷挂载失败：

- 确保测试目录有写权限。
- macOS/Windows 环境下检查 Docker 文件共享设置。

GPU 测试失败：

- 确保已安装 NVIDIA Container Toolkit。
- 检查 `nvidia-smi` 是否可用。
- 验证 Docker GPU 配置：

```bash
docker run --rm --gpus all nvidia/cuda:12.1.1-base-ubuntu22.04 nvidia-smi
```

## 新增测试约定

- 测试脚本以 `test_` 开头。
- 使用 Bash 脚本并添加执行权限。
- 包含 `trap cleanup EXIT` 形式的清理逻辑。
- 输出清晰的测试步骤和失败原因。
