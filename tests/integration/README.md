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
