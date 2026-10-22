"""Opt-in acceptance for the frozen project-first UI; never starts GPU work.

Uses only its own synthetic project/dataset. The caller selects an authorized,
idle node/card list. Reports actual telemetry without credentials or user data.
"""
import argparse
import copy
import json
from pathlib import Path

import requests


def main():
    parser = argparse.ArgumentParser()
    for name in ('base-url', 'teacher-url', 'worker', 'model-id', 'gpus', 'report'):
        parser.add_argument('--' + name, required=True)
    parser.add_argument('--uid', type=int, required=True)
    args = parser.parse_args()
    base = args.base_url.rstrip('/') + '/api/v1'
    indices = [int(x) for x in args.gpus.split(',')]
    checks = []

    def call(method, path, expected=200, **kwargs):
        kwargs.setdefault('params', {}).setdefault('uid', args.uid)
        response = requests.request(method, base + path, timeout=40, **kwargs)
        assert response.status_code == expected, (method, path, response.status_code, response.text)
        if expected == 200:
            return response.json().get('data')

    runtime = call('GET', '/runtime')
    assert runtime['training_scope'] == 'single_node_multi_gpu' and runtime['max_training_nodes'] == 1
    catalog = call('GET', '/resources/available')
    node = next(x for x in catalog['nodes'] if x['name'] == args.worker)
    assert node['training_supported'] and node['data_supported'] and node['observed_at']
    cards = {x['index']: x for x in node['devices']}
    for index in indices:
        assert cards[index]['selectable'] and cards[index]['state'] == 'available', cards[index]
        assert cards[index]['memory_total_mib'] > 0 and cards[index]['process_count'] == 0
    resource = {'selected_resources': [{'node_name': args.worker, 'xpu_indices': indices}], 'executor_nodes': [args.worker]}
    valid = call('POST', '/resources/validate', json=resource)
    assert valid['valid'] and not valid['reserves_resources']
    assert valid['resource_request']['gpu_count'] == len(indices)
    for bad in ({}, {'selected_resources': resource['selected_resources'] * 2},
                {'selected_resources': [{'node_name': args.worker, 'xpu_indices': []}]},
                {'selected_resources': [{'node_name': args.worker, 'xpu_indices': [indices[0], indices[0]]}]},
                {**resource, 'gpu_count': len(indices) + 1}):
        call('POST', '/resources/validate', expected=400, json=bad)
    call('POST', '/resources/validate', expected=409,
         json={'selected_resources': [{'node_name': args.worker, 'xpu_indices': [9999]}]})
    busy = next((x for x in node['devices'] if x['state'] == 'in_use'), None)
    if busy:
        call('POST', '/resources/validate', expected=409,
             json={'selected_resources': [{'node_name': args.worker, 'xpu_indices': [busy['index']]}]})
    checks.append('real_card_telemetry_and_single_node_selection')

    project = call('POST', '/projects', json={'uid': args.uid, 'name': '固定页面接口验收（合成）'})
    project_id = project['id']
    call('PUT', '/projects/' + project_id, json={'uid': args.uid, 'name': '固定页面接口验收（已编辑）',
         'teacher_model_config': {'endpoint': args.teacher_url, 'model_name': 'fixture', 'api_secret_ref': 'fixture'}})
    for filename, content in [('empty.jsonl', ''), ('invalid.jsonl', '{bad')]:
        call('POST', '/datasets/upload', expected=400, data={'uid': args.uid, 'name': 'invalid'}, files={'file': (filename, content)})
    row = {'question': 'What is 2+2?', 'problem': 'What is 2+2?', 'response': 'Four.',
           'persona': 'A helpful assistant', 'background': 'Synthetic arithmetic',
           'images': ['https://example.invalid/synthetic.png'], 'metadata': {'synthetic': True}}
    dataset = call('POST', '/datasets/upload', data={'uid': args.uid, 'name': '固定页面字段映射验收'},
                   files={'file': ('seed.jsonl', json.dumps(row))})
    dataset_id = dataset['id']
    preview = call('GET', '/datasets/' + dataset_id + '/preview')
    assert preview['total'] == 1 and 'question' in preview['fields']
    models = call('GET', '/models/student')
    assert next(x for x in models if x['id'] == args.model_id)['ready']
    call('GET', '/teachers')
    teacher = call('POST', '/teachers/test', json={'uid': args.uid, 'teacher_model': {
        'endpoint': args.teacher_url, 'model_name': 'fixture', 'api_secret_ref': 'fixture'}})
    assert teacher['connected']
    checks.append('projects_datasets_mapping_teacher_student_catalogs')

    recipes = call('GET', '/workflows')['items']
    assert len(recipes) == 15
    payload = {'uid': args.uid, 'project_id': project_id, 'dataset_id': dataset_id,
               'training_config': {'distillation': {'version': '2.0', 'job_type': 'instruct_distill',
                    'name': '接口配置原本', 'train_student': False, 'field_mapping': {'instruction': 'question'},
                    'teacher_model': {'endpoint': args.teacher_url, 'model_name': 'fixture', 'api_secret_ref': 'fixture',
                                      'max_tokens': 2048, 'concurrency': 2}}}, 'resource_request': resource}
    jobs = ['instruct_distill', 'advanced_instruct_distill', 'augmented_instruct_distill', 'balanced_instruct_distill',
            'cot_distill', 'advanced_cot_distill', 'cot_long2short', 'cot_short2long',
            'dpo_data_build', 'agent_distill', 'mm_instruct_distill']
    # Read the server directory rather than inventing recipe IDs/defaults.
    directory = {x['job_type']: x for x in recipes}
    for job in jobs:
        body = copy.deepcopy(payload)
        body['training_config']['distillation']['job_type'] = job
        body['training_config']['distillation']['parameters'] = directory[job]['default_parameters']
        params = body['training_config']['distillation']['parameters']
        expected = {}
        for stage in params.get('pipeline', []):
            overrides = {
                'quality_filter': {'min_scores': {'helpfulness': 7, 'correctness': 1}, 'keep_top_ratio': 0.8},
                'cot_mix_by_rv_cd': {'cd_bins': [0, 4, 7, 10], 'rv_target': 'medium', 'min_correctness': 1},
                'generate_candidates': {'n': 3}, 'build_preference_pairs': {'min_margin': 1.5},
                'agent_trajectory': {'max_steps': 5, 'repeat_times': 2},
            }.get(stage['stage'])
            if overrides:
                stage['config'].update(overrides)
                expected[stage['stage']] = overrides
        plan = call('POST', '/workflows/preview', json=body)
        assert len(plan['stages']) == 4 and plan['resource_request']['gpu_count'] == 0
        assert not plan['resource_request'].get('selected_resources')
        assert plan['config']['backend']['api_key'] == '********'
        for stage in plan['config'].get('pipeline', []):
            for key, value in expected.get(stage['stage'], {}).items():
                assert stage['config'][key] == value, (job, stage)
            config = stage.get('config', {})
            if 'max_tokens' in config:
                assert config['max_tokens'] == 2048, (job, stage)
            if 'max_workers' in config:
                assert config['max_workers'] == 2, (job, stage)
    checks.append('eleven_frozen_ui_recipes_preview')

    train = copy.deepcopy(payload)
    train['training_config'].update({'num_train_epochs': 0.5, 'learning_rate': 0.0001, 'per_device_train_batch_size': 1})
    train['training_config']['distillation'].update({'train_student': True, 'student_model': {'model_id': args.model_id}, 'finetuning_type': 'lora'})
    plan = call('POST', '/workflows/preview', json=train)
    assert len(plan['stages']) == 5 and plan['resource_request']['gpu_count'] == len(indices)
    source = call('POST', '/pipelines', json=train)
    source_id = source['id']
    clone = call('POST', '/pipelines/' + source_id + '/clone', json={
        'name': '编辑后的副本', 'training_config': {'num_train_epochs': 1.5, 'distillation': {'train_student': False}},
        'resource_request': {'executor_nodes': [args.worker]}})
    assert clone['status'] == 'pending' and clone['project_id'] == project_id
    assert clone['training_config']['num_train_epochs'] == 1.5
    assert clone['resource_request']['gpu_count'] == 0
    assert clone['training_config']['distillation']['teacher_model']['api_secret_ref'] == '********'
    original = call('GET', '/pipelines/' + source_id, params={'project_id': project_id})
    assert original['training_config']['num_train_epochs'] == 0.5 and original['training_config']['distillation']['train_student']
    call('GET', '/pipelines/' + source_id, expected=404, params={'project_id': 'another-project'})
    call('GET', '/pipelines/' + source_id, expected=404, params={'uid': args.uid + 100000})
    call('POST', '/pipelines/' + source_id + '/clone', expected=400, json={'project_id': 'another-project'})
    call('POST', '/pipelines/' + source_id + '/cancel')
    call('POST', '/pipelines/' + clone['id'] + '/cancel')
    assert call('GET', '/pipelines', params={'project_id': project_id})['total'] == 2
    checks.append('fractional_epochs_editable_clone_isolation_pending_cancel')
    call('DELETE', '/projects/' + project_id)
    call('DELETE', '/datasets/' + dataset_id)
    Path(args.report).write_text(json.dumps({'result': 'FRONTEND_CONTRACT_OK', 'checks': checks,
        'worker': args.worker, 'gpus': indices, 'cards': [cards[x] for x in indices]}, ensure_ascii=False, indent=2))
    print('FRONTEND_CONTRACT_OK', flush=True)


if __name__ == '__main__':
    main()
