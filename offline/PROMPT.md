# gcs-distill 离线包制作提示词

请从已经健康运行的 gcs-distill 版本生成或更新本项目自己的离线部署目录。

1. 29.80 保留完整 x86 源码并作为功能更新基线；43 只保留 ARM 已编译运行文件。二进制、镜像、日志、临时文件和备份不提交 Git。
2. ARM 服务先在 `/gcs-distill` 验证健康和业务链路，再按白名单生成 `/gcs-distill/offline`。不要增加 `make offline` 业务目标，也不要递归复制已有离线目录。
3. 离线目录只复制 ARM 二进制、43 配置案例、BUILD-INFO、Makefile、systemd 模板、README、PROMPT、镜像加载脚本、独立测试脚本和指定镜像归档。
4. 指定镜像为 `gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64`。使用 `docker save` 导出为 `images/gcs-distill-easydistill-ascend-0.22.1rc1-cann9.0-arm64.tar`，不生成校验文件。
5. `executor.runtime_image` 必须与归档加载标签完全一致。所有模型、数据和输出都使用现场本地绝对路径，不访问 Hugging Face。
6. 离线目录可以改名和移动。`make install` 只依据当前真实目录生成 systemd unit，不复制应用文件到 `/usr/local/bin`、`/opt`、`/gcs-distill` 等固定目录。
7. 每台可调度 worker 都先执行 `make load-image`。910B 现场运行 `scripts/test-image.sh`，必须真实完成教师推理、学生训练和评估并返回成功标记。
8. 910A 验收项目、数据集、Pipeline CRUD、阶段提交、节点/NPU 透传、状态、日志、取消和删除；910B 验收镜像真实执行。
9. 不复制 MySQL 数据、历史任务、模型、数据集、训练输出、共享存储业务数据、Redis 状态、日志、容器、资源占用状态、源码或备份。
10. 只通过无冲突 PR 提交源码、脚本、模板和文档；镜像 tar、二进制与生成离线目录只保存在对应服务器。
