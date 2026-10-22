# EasyDistill 2 acceptance

`easydistill2_smoke.py` runs four official jobs against a deterministic local teacher inside the data image with `--network none`. It validates runtime behavior, not model quality.

`easydistill2_control_smoke.py --base-url URL --uid UID --teacher-url URL --worker NAME --report FILE` is an explicit integration run against the real control service, database and GCS worker. Use synthetic data only. It creates and removes its own labeled project/datasets on success, retaining failures for diagnosis. The teacher URL must expose the fixture (`easydistill2_smoke.py serve 18882`) and be reachable from the worker. GPU training is a separate opt-in test.

Run the scripts inside the supplied images to avoid host pip dependencies. Substitute real internal addresses and an allocated test UID:

```sh
TESTS="$PWD/tests/integration" # use "$PWD/tests" at the offline bundle root
docker run --rm --pull=never --network none -v "$TESTS:/tests:ro" gcs/distill-data:2.0.0-aeb8ca3-x86_64 python /tests/easydistill2_smoke.py
docker run --rm --pull=never --network host -v "$TESTS:/tests:ro" -v /absolute/report-dir:/reports gcs/distill-data:2.0.0-aeb8ca3-x86_64 python /tests/easydistill2_control_smoke.py --base-url http://CONTROL:8080 --uid TEST_UID --teacher-url http://FIXTURE:18882/v1 --worker WORKER --report /reports/control.json
```

The first command starts its own fixture inside the isolated container. For the platform command, start a reachable fixture separately using the same data image and `python /tests/easydistill2_smoke.py serve 18882`; stop that temporary container after acceptance.

`llamafactory_offline_smoke.py --device cuda` runs native SFT and DPO for one step each. Mount a complete local base model at `/model:ro` and an empty writable directory at `/work`, select one available GPU, and pass `--network none` to Docker. It asserts that only one GPU is visible.

`easydistill2_student_smoke.py` uses the real platform and additionally requires `--model-id`, `--gpu`, and optionally `--gpus 6,7` for a two-GPU DDP run. It checks all five stages and downloads a ZIP containing real adapter weights. Use only GPUs explicitly allocated to acceptance. These checks verify functionality, not model quality.

## 固定页面接口验收

`frontend_contract_smoke.py` 对运行中的控制服务验证项目、数据、教师/学生目录、页面 11 个配方预览、原生参数落地、可编辑复用配置，以及单机多卡资源合同。它创建并清理自己的合成记录，不启动 GPU 工作。使用真实空闲卡调用资源校验（不预留），并验证繁忙卡被拒绝。

```sh
python frontend_contract_smoke.py --base-url http://CONTROL:8080 --uid UID --teacher-url http://WORKER:18882/v1 --worker NODE --model-id MODEL_DIRECTORY_NAME --gpus 6,7 --report /reports/frontend-contract.json
```

教师 fixture 沿用上文数据验收脚本的 `serve` 子命令；脚本可在已交付数据镜像内运行，宿主机无需安装 Python 依赖。节点/卡号必须替换为已授权且空闲的实际资源。

## Agent 和图文真实模型验收

`easydistill2_agent_mm_smoke.py` 的 `--mode agent/mm/mm-cot` 分别验证原生 Agent 六步、图文指令和图文 CoT。通过真实 GCS 生成数据、登记资产、预览、学生 LoRA 及下载权重；Agent 要求生成样本含工具调用/返回，图文要求附件与标记均存在。使用合成种子及真实教师，成功后只清理本次创建的数据库记录，失败时保留以供诊断。

```sh
python easydistill2_agent_mm_smoke.py --mode agent --base-url http://CONTROL:8080 --uid UID --worker NODE --student Qwen2.5-0.5B-Instruct --gpu 7 --teacher-service-id OWNED_TEACHER_ID --report /reports/agent.json
python easydistill2_agent_mm_smoke.py --mode mm --base-url http://CONTROL:8080 --uid UID --worker NODE --student Qwen2-VL-2B-Instruct --gpu 7 --teacher-url http://TEACHER:18883/v1 --teacher-model Qwen2-VL-2B-Instruct --image /absolute/user/path/red-square.png --image /absolute/user/path/blue-circle.png --report /reports/mm.json
```

测试图片可由 `easydistill2_vision_teacher.py --model-path /model --image-dir /fixtures` 创建；该脚本使用真实本地 Qwen2-VL 权重，提供仅验收用的 OpenAI 接口，默认端口 18883、测试密钥 acceptance。请放在隔离验收网络中，限定一张已授权的 GPU；完成后停止临时教师。生产服务应通过模型中心管理。教师使用一张卡，训练另用一张，不同时抢占同一卡。

`verify_distill_checkpoints.py REPORT... --report OUTPUT` 在训练镜像内以 CPU、断网运行，需只读挂载报告对应的用户任务目录；验证优化步数、有限正 loss、所有适配器张量无 NaN/Inf 且 lora_B 有实际非零更新。教师/学生同为 2B 的图文案例只验证功能，不证明压缩或效果提升。
