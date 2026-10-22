"""Explicit GPU acceptance through the real platform using synthetic data."""
import argparse
import io
import json
import time
import zipfile
from pathlib import Path
import requests

p = argparse.ArgumentParser()
for name in ['base-url', 'teacher-url', 'worker', 'model-id', 'report']:
    p.add_argument('--' + name, required=True)
p.add_argument('--uid', type=int, required=True)
p.add_argument('--gpu', type=int, required=True)
p.add_argument('--gpus',default='')
a = p.parse_args()
indices=[int(x) for x in a.gpus.split(',')] if a.gpus else [a.gpu]
base = a.base_url.rstrip('/') + '/api/v1'
def req(method, path, **kw):
    kw.setdefault('params', {}).setdefault('uid', a.uid)
    r = requests.request(method, base + path, timeout=60, **kw)
    if r.status_code >= 400:
        raise RuntimeError((r.status_code, r.text))
    return r.json().get('data')
project = req('POST', '/projects', json={'uid': a.uid, 'name': 'EasyDistill 2 学生训练验收', 'teacher_model_config': {'endpoint': a.teacher_url, 'model_name': 'fixture', 'api_secret_ref': 'fixture'}})
seed = '\n'.join(json.dumps({'instruction': 'Explain two plus two, example ' + str(i)}) for i in range(2))
dataset = req('POST', '/datasets/upload', data={'uid': a.uid, 'name': 'GPU 验收合成数据'}, files={'file': ('seed.jsonl', seed)})
report = []
for job in ['instruct_distill', 'dpo_data_build']:
    body = {'uid': a.uid, 'project_id': project['id'], 'dataset_id': dataset['id'],
            'training_config': {'num_train_epochs': 1, 'per_device_train_batch_size': 1, 'gradient_accumulation_steps': 1,
                                'learning_rate': 0.0001, 'max_length': 128, 'logging_steps': 1, 'save_steps': 100,
                                'distillation': {'version': '2.0', 'job_type': job, 'train_student': True,
                                                 'student_model': {'model_id': a.model_id},
                                                 'model_template': 'qwen', 'finetuning_type': 'lora'}},
            'resource_request': {'executor_nodes': [a.worker], 'selected_resources': [{'node_name': a.worker, 'xpu_indices': indices}]}}
    preview = req('POST', '/workflows/preview', json=body)
    assert len(preview['stages']) == 5
    run = req('POST', '/pipelines', json=body)
    runid = run['id']
    print('STARTED', job, runid, flush=True)
    req('POST', '/pipelines/' + runid + '/start')
    deadline = time.time() + 600
    while time.time() < deadline:
        current = req('GET', '/pipelines/' + runid)
        if current['status'] in ['succeeded', 'failed', 'canceled']:
            break
        time.sleep(2)
    assert current['status'] == 'succeeded', current
    assert current['summary']['has_model_weights'] and current['summary']['output_records'] > 0
    stages = req('GET', '/pipelines/' + runid + '/stages')
    assert len(stages) == 5 and all(s['status'] == 'succeeded' for s in stages)
    artifact = requests.get(base + '/pipelines/' + runid + '/model/download', params={'uid': a.uid}, timeout=60)
    assert artifact.status_code == 200
    z = zipfile.ZipFile(io.BytesIO(artifact.content))
    assert 'adapter_model.safetensors' in z.namelist() and z.getinfo('adapter_model.safetensors').file_size > 0
    assert all(not name.startswith('configs/') and not name.endswith('.env') for name in z.namelist())
    report.append({'job_type': job, 'pipeline_id': runid, 'output_dataset_id': current['summary']['output_dataset_id'], 'gpus': indices,
                   'weights_bytes': z.getinfo('adapter_model.safetensors').file_size, 'status': 'passed'})
    print(json.dumps(report[-1]), flush=True)
Path(a.report).write_text(json.dumps({'result': 'EASYDISTILL2_STUDENT_GCS_GPU_OK', 'tests': report}, indent=2))
req('DELETE', '/projects/' + project['id'])
req('DELETE', '/datasets/' + dataset['id'])
for item in report:
    req('DELETE', '/datasets/' + item['output_dataset_id'])
print('EASYDISTILL2_STUDENT_GCS_GPU_OK', flush=True)
