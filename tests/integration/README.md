# GCS-Distill 集成测试

本目录包含 GCS-Distill 项目的集成测试脚本，用于验证 EasyDistill 容器集成和端到端蒸馏流程。

## 测试脚本

### 1. EasyDistill 容器测试 (`test_easydistill.sh`)

验证 EasyDistill Docker 镜像的基本功能。

**测试内容：**
- ✅ 镜像是否存在
- ✅ 镜像基本功能（--help）
- ✅ 工作空间卷挂载
- ✅ 日志文件写入到 `/workspace/logs/`
- ✅ 模型输出目录可访问性
- ✅ GPU 支持（如果可用）
- ✅ EasyDistill 命令可用性

**运行方式：**
```bash
# 先构建镜像
make docker-build

# 运行测试
./tests/integration/test_easydistill.sh
```

**预期输出：**
```
==========================================
EasyDistill 容器集成测试
==========================================

1. 检查 EasyDistill 镜像...
✓ 镜像存在

2. 测试镜像基本功能...
✓ 镜像可以正常运行

3. 测试工作空间挂载...
✓ 卷挂载正常

4. 测试日志输出到文件...
✓ 日志文件写入成功
日志内容: Test log output to file

5. 测试模型输出目录...
✓ 模型输出目录可访问

6. 测试 GPU 支持...
✓ GPU 支持正常

7. 测试 EasyDistill 命令...
✓ EasyDistill 命令可用

==========================================
所有测试通过！
==========================================
```

### 2. 端到端工作流测试 (`test_e2e_workflow.sh`)

模拟完整的蒸馏流程，验证工作空间结构、配置生成、日志和模型输出。

**测试内容：**
- ✅ 工作空间目录结构创建
- ✅ 种子数据准备
- ✅ 配置文件生成（教师推理、学生训练）
- ✅ 教师推理输出模拟
- ✅ 日志文件持久化
- ✅ 日志文件读取
- ✅ 模型检查点输出
- ✅ Docker 卷挂载验证

**运行方式：**
```bash
./tests/integration/test_e2e_workflow.sh
```

**预期输出：**
```
==========================================
端到端蒸馏流程验证
==========================================

1. 创建工作空间结构...
✓ 工作空间创建完成

2. 准备种子数据...
✓ 种子数据已复制

3. 生成教师推理配置...
✓ 教师推理配置已生成

4. 模拟教师推理输出...
✓ 教师推理输出已生成

5. 创建模拟日志文件...
✓ 日志文件已创建

6. 验证日志文件可读性...
✓ 日志文件可访问

7. 生成学生训练配置...
✓ 学生训练配置已生成

8. 模拟模型检查点输出...
✓ 模型检查点可访问

9. 测试 Docker 卷挂载...
✓ Docker 卷挂载正常

==========================================
端到端测试完成！
==========================================
```

## 测试数据

### sample_seed_data.json

包含 5 条示例种子指令，用于测试蒸馏流程。

**格式：**
```json
[
  {
    "instruction": "解释什么是机器学习",
    "input": "",
    "output": ""
  }
]
```

## 快速开始

```bash
# 1. 构建 EasyDistill 镜像
make docker-build

# 2. 测试镜像
make docker-test

# 3. 运行容器集成测试
./tests/integration/test_easydistill.sh

# 4. 运行端到端工作流测试
./tests/integration/test_e2e_workflow.sh
```

## 故障排查

### 镜像构建失败

```bash
# 检查 Docker 是否运行
docker ps

# 重新构建镜像
docker build -t gcs-distill/easydistill:latest -f docker/easydistill/Dockerfile .
```

### 卷挂载失败

- 确保测试目录有写权限
- 检查 Docker 的文件共享设置（macOS/Windows）

### GPU 测试失败

- 确保安装了 NVIDIA Docker Runtime
- 检查 `nvidia-smi` 是否可用
- 验证 Docker 配置：`docker run --rm --gpus all nvidia/cuda:12.1.1-base-ubuntu22.04 nvidia-smi`

## 持续集成

这些测试可以集成到 CI/CD 流水线中：

```yaml
# .github/workflows/test.yml 示例
- name: Build EasyDistill Image
  run: make docker-build

- name: Run Integration Tests
  run: |
    ./tests/integration/test_easydistill.sh
    ./tests/integration/test_e2e_workflow.sh
```

## 贡献

添加新测试时，请遵循以下约定：
1. 测试脚本以 `test_` 开头
2. 使用 Bash 脚本并添加执行权限
3. 包含清理函数（trap cleanup EXIT）
4. 使用颜色输出（GREEN/RED/YELLOW）
5. 提供清晰的测试描述和预期结果
