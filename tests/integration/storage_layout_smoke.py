"""Read-only verification of a completed training run on the actual shared mount."""
import argparse
import json
from pathlib import Path

import requests

parser = argparse.ArgumentParser()
parser.add_argument('--base-url', required=True)
parser.add_argument('--shared-root', type=Path, required=True)
parser.add_argument('--uid', type=int, required=True)
parser.add_argument('--other-uid', type=int, required=True, help='Existing different user for scope checks')
parser.add_argument('--pipeline-id', required=True)
parser.add_argument('--report', type=Path, required=True)
args = parser.parse_args()
assert args.uid > 0 and args.other_uid > 0 and args.uid != args.other_uid
base = args.base_url.rstrip('/') + '/api/v1'
user = args.shared_root / f'user-{args.uid}'
private_models = user / 'train-center/model-distill/models/base'


def call(path, uid=args.uid, expected=200):
    response = requests.get(base + path, params={'uid': uid}, timeout=40)
    assert response.status_code == expected, (path, response.status_code, response.text[:500])
    if expected == 200:
        return response.json()['data']


models = call('/models/student')
assert models and all(Path(model['path']).parent == private_models for model in models)
assert requests.get(base + '/models/student', timeout=30).status_code == 400
pipeline = call('/pipelines/' + args.pipeline_id)
assert pipeline['status'] == 'succeeded'
call('/pipelines/' + args.pipeline_id, uid=args.other_uid, expected=404)
root = user / f"train-center/model-distill/projects/{pipeline['project_id']}/runs/{args.pipeline_id}"
assert root.is_dir() and root.resolve() == root
run = pipeline['training_config']['distillation']
student = run['student_model']
assert Path(student['model_path']).parent == private_models
config = json.loads((root / 'configs/student_train.json').read_text())
assert Path(config['model_name_or_path']) == Path(student['model_path'])
assert Path(config['output_dir']) == root / 'models/checkpoints'
assert Path(config['logging_dir']).is_relative_to(root / 'logs')
for relative in ['data/seed/input.jsonl', 'data/exported/dataset.jsonl', 'data/exported/manifest.json']:
    assert (root / relative).is_file(), relative
for relative in ['data/intermediate', 'models/checkpoints', 'cache/huggingface/datasets', 'cache/home', 'cache/torch', 'tmp']:
    assert (root / relative).is_dir(), relative
assert any(path.suffix in ('.safetensors', '.bin') and path.stat().st_size > 0
           for path in (root / 'models/checkpoints').iterdir())
stages = call('/pipelines/' + args.pipeline_id + '/stages')
assert len(stages) == 5
for stage in stages:
    path = Path(stage['log_path'])
    assert path.parent == root / 'logs' and path.name == stage['stage_type'], path
    events = [json.loads(line) for line in (path / 'events.log').read_text().splitlines()]
    assert events[0]['status'] == 'started' and events[-1]['status'] == 'succeeded'
    response = requests.get(base + f"/pipelines/{args.pipeline_id}/stages/{stage['id']}/logs/download",
                            params={'uid': args.uid}, timeout=40)
    assert response.status_code == 200 and response.content, (stage['stage_type'], response.status_code)
manifest = json.loads((root / 'data/exported/manifest.json').read_text())
assert Path(manifest['dataset_path']).is_relative_to(root)
asset = Path(manifest['asset_path'])
assert asset.is_relative_to(user / 'train-center/model-distill/datasets/generated') and asset.is_file()
runtime = call('/runtime')
assert runtime['weights_source'] == 'user_private_model_pool'
result = {'result': 'USER_STORAGE_LAYOUT_OK', 'uid': args.uid, 'pipeline_id': args.pipeline_id,
          'workspace': str(root), 'student_model': student['model_path'], 'generated_dataset': str(asset),
          'all_five_stage_logs_downloadable': True, 'cross_user_rejected': True,
          'private_model_count': len(models), 'cache_directories': 'under run/cache', 'temporary_directory': str(root / 'tmp')}
args.report.parent.mkdir(parents=True, exist_ok=True)
args.report.write_bytes((json.dumps(result, indent=2) + '\n').encode('utf-8'))
print(json.dumps(result, indent=2), flush=True)
