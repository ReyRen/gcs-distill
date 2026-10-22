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
