# EasyDistill Dockerfile

该目录提供 gcs-distill 使用的 EasyDistill 镜像构建起点。

## 设计目标

- 作为 worker 节点任务容器的基础镜像
- 支持通过 `easydistill --config <config>` 直接运行
- 预留共享存储挂载目录 `/workspace`

## 目录约定

- `/workspace/configs`：运行配置
- `/workspace/data`：输入数据
- `/workspace/output`：输出模型和中间产物
- `/workspace/logs`：日志

## 构建示例

```bash
docker build -t gcs-distill/easydistill:latest -f docker/easydistill/Dockerfile .
```

## 运行示例

```bash
docker run --rm \
  -v /your/shared/path:/workspace \
  gcs-distill/easydistill:latest \
  --config /workspace/configs/job.json
```

## 说明

- 当前 Dockerfile 以 CUDA Runtime 镜像为基础，适合 GPU 场景起步。
- 如果后续要支持 Ascend NPU，需要单独派生对应基础镜像并安装配套驱动依赖。
- EasyDistill 的部分能力依赖更重的训练与推理栈，实际生产镜像还需要结合你选定的训练框架、CUDA 版本、驱动版本和模型依赖进一步固化。