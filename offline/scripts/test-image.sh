#!/usr/bin/env bash
set -euo pipefail

image="${IMAGE:-gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64}"
teacher_model="${1:?usage: test-image.sh TEACHER_MODEL_PATH [PHYSICAL_NPU_IDS] [STUDENT_MODEL_PATH] [WORKSPACE]}"
npu_ids="${2:-}"
student_model="${3:-$teacher_model}"
workspace="${4:-/tmp/gcs-distill-image-test-$$}"
mock_port="${MOCK_PORT:-38991}"
mock_container="gcs-distill-eval-mock-$$"

for tool in docker curl python3; do
  command -v "$tool" >/dev/null || { echo "Missing required command: $tool" >&2; exit 2; }
done
for model_path in "$teacher_model" "$student_model"; do
  [[ "$model_path" = /* && -d "$model_path" ]] || {
    echo "Model path must be an existing absolute directory: $model_path" >&2
    exit 2
  }
done
[[ "$workspace" = /* ]] || { echo 'WORKSPACE must be absolute' >&2; exit 2; }
available_npu_ids="$(find /dev -maxdepth 1 -type c -name 'davinci[0-9]*' -printf '%f\n' \
  | sed 's/^davinci//' | sort -n | paste -sd, -)"
[ -n "$available_npu_ids" ] || {
  echo 'No physical Ascend device nodes were found under /dev.' >&2
  exit 2
}
if [ -z "$npu_ids" ]; then
  npu_ids="${available_npu_ids%%,*}"
  echo "Auto-selected physical NPU $npu_ids from: $available_npu_ids"
fi
[[ "$npu_ids" =~ ^[0-9]+(,[0-9]+)*$ ]] || {
  echo "PHYSICAL_NPU_IDS must look like 0 or 0,1" >&2
  exit 2
}
IFS=',' read -r -a selected_npu_ids <<<"$npu_ids"
for npu_id in "${selected_npu_ids[@]}"; do
  [ -c "/dev/davinci$npu_id" ] || {
    echo "Physical NPU $npu_id does not exist. Available IDs: $available_npu_ids" >&2
    exit 2
  }
done

cleanup() {
  docker logs "$mock_container" >"$workspace/eval-mock.log" 2>&1 || true
  docker rm -f "$mock_container" >/dev/null 2>&1 || true
}
trap cleanup EXIT

rm -rf "$workspace"
mkdir -p "$workspace"/{configs,data/seed,data/generated,data/filtered,models/checkpoints,eval}
python3 - "$workspace" "$teacher_model" "$student_model" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
teacher, student = sys.argv[2:4]
template = root / 'chat_template_kd.jinja'
template.write_text(
    "{{ message.content }}{% if add_output %}{{ message.output }}{% endif %}",
    encoding='utf-8',
)
(root / 'data/seed/instructions.json').write_text(
    json.dumps([{'instruction': 'Answer with one word: the sky is'}]),
    encoding='utf-8',
)
teacher_config = {
    'job_type': 'kd_black_box_local',
    'dataset': {
        'instruction_path': str(root / 'data/seed/instructions.json'),
        'labeled_path': str(root / 'data/generated/labeled.json'),
        'template': str(template),
        'seed': 42,
    },
    'models': {'teacher': teacher, 'student': student},
    'inference': {
        'enable_chunked_prefill': False,
        'seed': 777,
        'gpu_memory_utilization': 0.50,
        'temperature': 0.0,
        'trust_remote_code': True,
        'enforce_eager': True,
        'max_model_len': 512,
        'max_new_tokens': 8,
    },
}
train_config = {
    'job_type': 'kd_black_box_train_only',
    'dataset': {
        'labeled_path': str(root / 'data/filtered/train.json'),
        'template': str(template),
        'seed': 42,
    },
    'models': {'teacher': teacher, 'student': student},
    'training': {
        'output_dir': str(root / 'models/checkpoints'),
        'num_train_epochs': 1,
        'per_device_train_batch_size': 1,
        'gradient_accumulation_steps': 1,
        'max_length': 128,
        'save_steps': 1,
        'logging_steps': 1,
        'learning_rate': 2e-5,
        'weight_decay': 0.0,
        'warmup_ratio': 0.0,
        'lr_scheduler_type': 'linear',
    },
}
(root / 'configs/teacher_infer.json').write_text(json.dumps(teacher_config, indent=2), encoding='utf-8')
(root / 'configs/student_train.json').write_text(json.dumps(train_config, indent=2), encoding='utf-8')
(root / 'mock_openai.py').write_text(r'''
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def reply(self, payload):
        body = json.dumps(payload).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)
    def do_GET(self):
        self.reply({'object': 'list', 'data': [{'id': 'gcs-test-judge', 'object': 'model'}]})
    def do_POST(self):
        length = int(self.headers.get('Content-Length', '0'))
        self.rfile.read(length)
        self.reply({'id': 'gcs-test', 'object': 'chat.completion', 'choices': [
            {'index': 0, 'message': {'role': 'assistant', 'content': '<score>1</score>'}, 'finish_reason': 'stop'}
        ]})
    def log_message(self, *_):
        pass

HTTPServer(('0.0.0.0', int(sys.argv[1])), Handler).serve_forever()
''', encoding='utf-8')
PY

mounts=(-v "$workspace:$workspace:rw" -v "$teacher_model:$teacher_model:ro")
if [[ "$student_model" != "$teacher_model" ]]; then
  mounts+=(-v "$student_model:$student_model:ro")
fi
common=(
  --pull=never
  --runtime=ascend --privileged --ipc=host --cap-add=IPC_LOCK --shm-size=32g
  -e ASCEND_VISIBLE_DEVICES="$npu_ids"
  -e HF_HUB_OFFLINE=1
  -e TRANSFORMERS_OFFLINE=1
  -e HF_HUB_DISABLE_TELEMETRY=1
  "${mounts[@]}"
  --workdir "$workspace"
)

echo '[1/4] Static inference, training and evaluation command contract'
docker run --rm --pull=never --entrypoint gcs-distill-contract "$image"

echo '[2/4] Run the exact local teacher-inference command used by gcs-distill'
docker run --rm --name "gcs-distill-infer-test-$$" \
  "${common[@]}" -e GCS_DISTILL_STAGE=teacher_infer \
  --entrypoint python "$image" \
  -m easydistill.kd.infer --config "$workspace/configs/teacher_infer.json"
python3 - "$workspace/data/generated/labeled.json" <<'PY'
import json, sys
data = json.load(open(sys.argv[1], encoding='utf-8'))
if not data or not data[0].get('output', '').strip():
    raise SystemExit('teacher inference did not produce a labeled response')
print('teacher output:', data[0]['output'].strip())
PY
cp "$workspace/data/generated/labeled.json" "$workspace/data/filtered/train.json"

echo '[3/4] Run the exact one-process student-training command used by gcs-distill'
accelerate_args=(launch)
npu_count="$(awk -F, '{print NF}' <<<"$npu_ids")"
if (( npu_count > 1 )); then accelerate_args+=(--multi_gpu); fi
accelerate_args+=(--num_processes "$npu_count" --module easydistill.kd.train --config "$workspace/configs/student_train.json")
docker run --rm --name "gcs-distill-train-test-$$" \
  "${common[@]}" -e GCS_DISTILL_STAGE=student_train \
  --entrypoint accelerate "$image" "${accelerate_args[@]}"
find "$workspace/models/checkpoints" -type f -size +0c -print -quit | grep -q . || {
  echo 'student training did not create checkpoint files' >&2
  exit 1
}

echo '[4/4] Run the EasyDistill evaluation command against a local OpenAI-compatible judge'
docker run -d --pull=never --name "$mock_container" -p "$mock_port:$mock_port" \
  -v "$workspace:$workspace:ro" --entrypoint python "$image" \
  "$workspace/mock_openai.py" "$mock_port" >/dev/null
deadline=$((SECONDS + 60))
until curl -fs "http://127.0.0.1:$mock_port/v1/models" >/dev/null 2>&1; do
  (( SECONDS < deadline )) || { docker logs "$mock_container" >&2; exit 1; }
  sleep 1
done
bridge_gateway="$(docker network inspect bridge --format '{{(index .IPAM.Config 0).Gateway}}')"
python3 - "$workspace" "http://$bridge_gateway:$mock_port/v1" <<'PY'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
config = {
    'job_type': 'cot_eval_api',
    'dataset': {
        'input_path': str(root / 'data/generated/labeled.json'),
        'output_path': str(root / 'eval/results.json'),
    },
    'inference': {'base_url': sys.argv[2], 'api_key': 'gcs-test', 'max_new_tokens': 8},
}
(root / 'configs/evaluate.json').write_text(json.dumps(config, indent=2), encoding='utf-8')
PY
docker run --rm --name "gcs-distill-eval-test-$$" \
  "${common[@]}" -e GCS_DISTILL_STAGE=evaluate \
  --entrypoint python "$image" \
  -m easydistill.eval.data_eval --config "$workspace/configs/evaluate.json"
test -s "$workspace/eval/results.json"

echo "DISTILL_IMAGE_TEST_OK workspace=$workspace"
