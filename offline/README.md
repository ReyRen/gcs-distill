# gcs-distill ARM64 可移动离线部署

本目录是现场唯一需要带走的交付物，可以在安装前放到任意不含空格的绝对路径并改名。目录内包含服务程序、43 当前配置案例、systemd 模板、EasyDistill Ascend 镜像归档及现场独立测试脚本。

ARM 配置沿用 43：服务监听 `172.18.127.43:8080`，MySQL 和 gcs-v2 位于 43，共享存储根为 `/storage-root-jfs`。运行镜像固定为 `gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64`。现场启动前根据实际网络、凭据和存储路径修改 `config.toml`。

## 现场依赖

- ARM64 Linux、Docker、Ascend Docker Runtime 和现场 910B 驱动/CANN 兼容环境；
- 已部署的 MySQL、gcs-v2、gcs-info-catch-v2 worker 和共享存储；
- 本地教师模型、学生模型和数据目录，不访问 Hugging Face；
- 每一台可能执行蒸馏阶段的 worker 都加载相同镜像。

## 加载并独立验证镜像

```bash
cd /现场最终目录
make load-image
bash scripts/test-image.sh /绝对路径/本地教师模型 0
```

第三个参数可以指定另一份本地学生模型。脚本按 GCS worker 的 privileged、host IPC、Ascend runtime 和物理卡选择协议，真实执行教师推理、单进程学生训练和本地模拟裁判评估。必须看到 `DISTILL_IMAGE_TEST_OK`，并且推理输出、checkpoint 和评估结果均实际生成。

## 安装和运维

```bash
make check
make install
make start
make verify
```

`make install` 只注册 `/etc/systemd/system/gcs-distill.service`，直接运行当前目录二进制。目录安装后若移动或改名，需要重新执行 `make install`。

后续统一使用 `make start`、`make stop`、`make restart`、`make status`、`make logs`、`make journal`、`make verify` 和 `make uninstall`。当前配置将日志写到 stdout，所以 `make logs` 和 `make journal` 都读取 systemd journal。

910A 环境验收项目、数据集、Pipeline CRUD、阶段提交、节点/NPU 选择、状态、日志、取消和删除；镜像真实推理、训练和评估由 910B 现场测试验收。

本包不迁移 MySQL 数据、历史任务、模型、数据集、训练输出、日志、容器或资源占用状态。
