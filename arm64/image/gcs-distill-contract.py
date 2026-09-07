#!/usr/bin/env python3
import json
import importlib.util
import pathlib
import platform
import py_compile
import subprocess
from importlib import metadata


if platform.machine() not in ("aarch64", "arm64"):
    raise SystemExit(f"unexpected architecture: {platform.machine()}")

infer_versions = {
    name: metadata.version(name)
    for name in ("torch", "torch_npu", "vllm", "vllm-ascend", "transformers", "openai", "jsonlines", "easydistill")
}
expected_prefixes = {
    "torch": "2.10.0",
    "torch_npu": "2.10.0",
    "vllm": "0.22.1",
    "vllm-ascend": "0.22.1rc1",
    "transformers": "5.5.4",
}
for name, prefix in expected_prefixes.items():
    if not infer_versions[name].startswith(prefix):
        raise SystemExit(f"inference {name}={infer_versions[name]} does not match {prefix}")

for module in ("torch_npu", "vllm", "vllm_ascend", "transformers"):
    if importlib.util.find_spec(module) is None:
        raise SystemExit(f"missing inference module: {module}")
source_root = pathlib.Path("/opt/gcs/easydistill/easydistill")
for relative in ("kd/infer.py", "kd/train.py", "eval/data_eval.py"):
    source = source_root / relative
    if not source.is_file():
        raise SystemExit(f"missing EasyDistill command module: {source}")
    py_compile.compile(str(source), doraise=True)

train_code = r'''
import json
from importlib import metadata
names = ("transformers", "tokenizers", "trl", "accelerate", "datasets", "easydistill")
print(json.dumps({name: metadata.version(name) for name in names}, sort_keys=True))
'''
completed = subprocess.run([
    "/opt/gcs/train-venv/bin/python", "-c", train_code
], check=True, text=True, capture_output=True)
train_versions = json.loads(completed.stdout.strip().splitlines()[-1])
for name, expected in {
    "transformers": "4.51.1",
    "tokenizers": "0.21.1",
    "trl": "0.17.0",
    "accelerate": "1.13.0",
    "datasets": "4.8.4",
}.items():
    if train_versions[name] != expected:
        raise SystemExit(f"training {name}={train_versions[name]} does not match {expected}")

print(json.dumps({
    "contract": "gcs-distill/stages-v1",
    "architecture": platform.machine(),
    "cann": "9.0.0",
    "target_hdk": "25.3.x",
    "commands": [
        "python -m easydistill.kd.infer",
        "accelerate launch --module easydistill.kd.train",
        "python -m easydistill.eval.data_eval",
    ],
    "inference": infer_versions,
    "training": train_versions,
}, indent=2, sort_keys=True))
