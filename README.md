# GCS Distill 2

x86 离线蒸馏控制服务。官方 EasyDistill 2 调用内网教师服务生成、评分、筛选和导出数据；独立 LLaMA-Factory 容器可选加载共享学生基模进行 SFT 或 DPO 训练。

`gcs-distill → gcs-v2 → gcs-info-catch-v2` 复用平台调度、GPU 分配、共享存储和日志。数据任务使用 CPU 容器，教师推理运行在已有服务，学生训练加载本地完整权重。旧 EasyDistill 1 推理、治理、训练和 ARM 适配已删除，旧任务不能重新启动。

```text
teacher_config → dataset_build → distill_data → dataset_export → [student_train]
```

任务保存教师、学生、配方与训练参数快照。生成数据属于用户，可跨项目复用。学生训练失败不会删除已导出的数据。权重和日志可下载，结束后的容器自动回收。

- [前端接口与页面交接](docs/easydistill2-frontend-handoff.md)
- [交互 mock](docs/ui/gcs-distill2-mock.html)
- [离线部署与基础环境](offline/README.md)
- [OpenAPI](server/apidocs/swagger/openapi.json)，服务入口 `/swagger/`

现有 x86 联调环境部署在 `172.18.29.80`，前端可直接使用：

| 用途 | 地址 |
|---|---|
| Swagger 页面，展开接口后可使用 Try it out | [接口展示](http://172.18.29.80:8080/swagger/index.html) |
| OpenAPI，可导入 Apifox/Postman | [接口定义](http://172.18.29.80:8080/swagger/openapi.json) |
| API 基础地址 | `http://172.18.29.80:8080/api/v1` |
| 健康检查 | [health](http://172.18.29.80:8080/health) |

先读取 `/workflows`、`/runtime`、`/models/student` 和 `/resources/available` 初始化表单。资源提交前调用 `/resources/validate`；创建任务为 pending，需另调 start。用户 UID 从平台登录态取得，项目、数据集与任务的详细对接规则见前端交接文档。

```sh
go test ./...
make build
GCS_DISTILL_DB_PASSWORD=... bin/gcs-distill-server --config config.toml
```

内部 API 必须位于可信平台认证网关之后。UID 来自登录态，任务、阶段、日志和下载均校验归属。不得允许外部用户任意伪造 UID。

用户目录为 `/storage-root-jfs/user-{uid}/train-center/model-distill`，用户私有学生基模目录为 `/storage-root-jfs/user-{uid}/train-center/model-distill/models/base`。含密钥配置不作为产物开放。运行时禁止拉取镜像、模型或安装依赖；缺少镜像时明确报错。

EasyDistill 固定提交 `aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3`；LLaMA-Factory 固定提交 `ca75f1edf3cb50343ed1c98605141c3e22075b5f`。构建阶段可联网，现场通过完整镜像归档部署。学生效果评估由独立测评模块完成，数据评分不代表模型效果。
