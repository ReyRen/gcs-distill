"""Real-teacher Agent/image-text acceptance through the platform.

Explicitly opt in with an authorized GPU. Synthetic seeds are used with a real
teacher; native Agent tools remain the upstream simulated-tool workflow.
"""
import argparse
import copy
import io
import json
from pathlib import Path
import time
import zipfile

import requests


def main():
    parser = argparse.ArgumentParser()
    for field in ['base-url', 'worker', 'student', 'report']:
        parser.add_argument('--' + field, required=True)
    parser.add_argument('--mode', choices=['agent', 'mm', 'mm-cot'], required=True)
    parser.add_argument('--uid', type=int, required=True)
    parser.add_argument('--gpu', type=int, required=True)
    parser.add_argument('--teacher-service-id')
    parser.add_argument('--teacher-url')
    parser.add_argument('--teacher-model')
    parser.add_argument('--teacher-key', default='acceptance')
    parser.add_argument('--image', action='append', default=[])
    args = parser.parse_args()
    base = args.base_url.rstrip('/') + '/api/v1'
    report = {'mode': args.mode, 'teacher': args.teacher_service_id or args.teacher_model,
              'student': args.student, 'gpu': args.gpu, 'result': 'running'}

    def save():
        Path(args.report).write_text(json.dumps(report, ensure_ascii=False, indent=2))

    def call(method, path, **kwargs):
        kwargs.setdefault('params', {}).setdefault('uid', args.uid)
        response = requests.request(method, base + path, timeout=60, **kwargs)
        response.raise_for_status()
        return response.json().get('data')

    teacher = {'service_id': args.teacher_service_id} if args.teacher_service_id else {
        'endpoint': args.teacher_url, 'model_name': args.teacher_model, 'api_secret_ref': args.teacher_key}
    teacher.update({'max_tokens': 4096 if args.mode == 'agent' else 256, 'concurrency': 1, 'timeout_seconds': 180})
    project = call('POST', '/projects', json={'uid': args.uid, 'name': f'{args.mode} 真实教师完整验收（合成种子）'})
    report['project_id'] = project['id']
    if args.mode == 'agent':
        seeds = [{'persona': 'A laboratory inventory analyst working with entirely synthetic stock records.',
                  'background': 'Reconcile laboratory stock counts, calculate a restocking summary and prepare a report. '
                                'Use small synthetic records and simple lookup, calculation and report tools. '
                                'All tools are virtual; no personal data, external systems, purchases or real-world side effects.'}]
        job = 'agent_distill'
    else:
        assert len(args.image) >= 2, 'Provide two local images to verify attachment preservation'
        seeds = [{'instruction': 'Describe the colored shape in the image in one sentence.', 'images': [path]} for path in args.image]
        job = 'mm_cot_distill' if args.mode == 'mm-cot' else 'mm_instruct_distill'
    dataset = call('POST', '/datasets/upload', data={'uid': args.uid, 'name': f'{args.mode} synthetic seeds'},
                   files={'file': ('seed.jsonl', '\n'.join(json.dumps(x) for x in seeds))})
    report['seed_dataset_id'] = dataset['id']
    recipe = next(x for x in call('GET', '/workflows')['items'] if x['job_type'] == job)
    parameters = copy.deepcopy(recipe['default_parameters'])
    parameters.setdefault('sft', {}).update({'min_length': 1, 'max_length': 65536})
    if args.mode == 'agent':
        parameters['agent'].update({'max_steps': 6, 'repeat_times': 2, 'max_tool_calls': 4})
        for stage in parameters['pipeline']:
            if stage['stage'] == 'agent_trajectory':
                stage['config'].update({'max_steps': 6, 'repeat_times': 2, 'max_tool_calls': 4})
    body = {'uid': args.uid, 'project_id': project['id'], 'dataset_id': dataset['id'],
            'training_config': {'num_train_epochs': 1, 'learning_rate': 0.0001, 'per_device_train_batch_size': 1,
                'gradient_accumulation_steps': 1, 'max_length': 4096 if args.mode == 'agent' else 1024,
                'logging_steps': 1, 'save_steps': 100, 'distillation': {
                    'version': '2.0', 'name': f'{args.mode} genuine teacher + student', 'job_type': job,
                    'parameters': parameters, 'teacher_model': teacher, 'train_student': True,
                    'student_model': {'model_id': args.student}, 'finetuning_type': 'lora'}},
            'resource_request': {'executor_nodes': [args.worker],
                'selected_resources': [{'node_name': args.worker, 'xpu_indices': [args.gpu]}]}}
    run = call('POST', '/pipelines', json=body)
    report['pipeline_id'] = run['id']
    save()
    print('CREATED', json.dumps(report), flush=True)
    call('POST', '/pipelines/' + run['id'] + '/start')
    deadline = time.time() + 1800
    previous = None
    while time.time() < deadline:
        current = call('GET', '/pipelines/' + run['id'])
        progress = (current['status'], current['current_stage'])
        if progress != previous:
            print('PROGRESS', progress, flush=True)
            previous = progress
        if current['status'] in ['succeeded', 'failed', 'canceled']:
            break
        time.sleep(3)
    stages = call('GET', '/pipelines/' + run['id'] + '/stages')
    report['stages'] = [{'stage': x['stage_type'], 'status': x['status'], 'error': x.get('error_message'),
                         'output': x.get('output_manifest')} for x in stages]
    report['status'] = current['status']
    report['result'] = 'failed'  # Publish passed only after artifact checks.
    save()
    assert current['status'] == 'succeeded', report
    assert len(stages) == 5 and all(x['status'] == 'succeeded' for x in stages)
    generated_id = current['summary']['output_dataset_id']
    output = call('GET', '/datasets/' + generated_id + '/preview', params={'limit': 100})
    assert output['total'] > 0
    rows = output['items']
    assert all(row.get('messages') for row in rows)
    if args.mode.startswith('mm'):
        assert all(row.get('images') for row in rows), 'Top-level images must survive data export'
        for row in rows:
            assert '<image>' in json.dumps(row['messages']), 'Training image marker is required'
    else:
        assert any('<tool_call>' in json.dumps(row['messages']) and '<tool_response>' in json.dumps(row['messages']) for row in rows), 'Require a real generated tool trajectory'
    response = requests.get(base + '/pipelines/' + run['id'] + '/model/download', params={'uid': args.uid}, timeout=120)
    response.raise_for_status()
    weights = zipfile.ZipFile(io.BytesIO(response.content)).getinfo('adapter_model.safetensors').file_size
    assert weights > 0
    report.update({'result': 'passed', 'output_records': output['total'], 'weights_bytes': weights,
                   'output_dataset_id': generated_id})
    save()
    call('DELETE', '/projects/' + project['id'])
    call('DELETE', '/datasets/' + dataset['id'])
    call('DELETE', '/datasets/' + generated_id)
    print('AGENT_MM_ACCEPTANCE_OK', json.dumps(report, ensure_ascii=False), flush=True)


if __name__ == '__main__':
    main()
