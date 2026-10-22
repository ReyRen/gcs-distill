"""Opt-in offline smoke: real SFT and DPO with a preinstalled small local model.

Run inside the trainer image with --network none, a read-only /model mount,
and a writable /work mount. Use --device cuda only with an explicitly allocated GPU. It does not test model quality.
"""
import argparse
import json
import os
import subprocess
from pathlib import Path

parser=argparse.ArgumentParser()
parser.add_argument('--device',choices=['cpu','cuda'],default='cpu')
args=parser.parse_args()
root = Path('/work')
root.mkdir(exist_ok=True)
os.environ['OMP_NUM_THREADS'] = '4'
os.environ['MKL_NUM_THREADS'] = '4'
if args.device=='cpu':
    os.environ['CUDA_VISIBLE_DEVICES'] = ''
else:
    import torch
    assert torch.cuda.is_available() and torch.cuda.device_count()==1, 'GPU isolation failed'
for stage in ['sft', 'dpo']:
    data = root / (stage + '.jsonl')
    rows = [{'messages': [{'role': 'user', 'content': 'What is 2+2?'}, {'role': 'assistant', 'content': 'Four.'}]}] * 2
    descriptor = {'file_name': str(data), 'formatting': 'sharegpt', 'columns': {'messages': 'messages'},
                  'tags': {'role_tag': 'role', 'content_tag': 'content', 'user_tag': 'user', 'assistant_tag': 'assistant'}}
    if stage == 'dpo':
        rows = [{'instruction': 'What is 2+2?', 'input': '', 'chosen': 'Four.', 'rejected': 'Five.'}] * 2
        descriptor = {'file_name': str(data), 'ranking': True,
                      'columns': {'prompt': 'instruction', 'query': 'input', 'chosen': 'chosen', 'rejected': 'rejected'}}
    data.write_text('\n'.join(json.dumps(x) for x in rows))
    (root / 'dataset_info.json').write_text(json.dumps({'gcs_distilled': descriptor}))
    output = root / (stage + '-weights')
    cfg = {'model_name_or_path': '/model', 'trust_remote_code': True, 'stage': stage, 'do_train': True,
           'finetuning_type': 'lora', 'lora_target': 'q_proj,v_proj', 'lora_rank': 4,
           'dataset': 'gcs_distilled', 'dataset_dir': str(root), 'template': 'qwen',
           'output_dir': str(output), 'report_to': 'none', 'overwrite_output_dir': False,
           'num_train_epochs': 1, 'max_steps': 1, 'per_device_train_batch_size': 1,
           'gradient_accumulation_steps': 1, 'learning_rate': 0.0001, 'cutoff_len': 32,
           'logging_steps': 1, 'save_steps': 10, 'fp16': args.device=='cuda', 'bf16': False,
           'use_cpu': args.device=='cpu', 'flash_attn': 'sdpa', 'save_safetensors': True,
           'dataloader_num_workers': 0, 'dataloader_pin_memory': False, 'disable_tqdm': True}
    path = root / (stage + '-config.json')
    path.write_text(json.dumps(cfg))
    subprocess.run(['llamafactory-cli', 'train', str(path)], check=True, timeout=600)
    weights = list(output.glob('adapter_model.safetensors'))
    assert weights and weights[0].stat().st_size > 0
    print(json.dumps({'stage': stage, 'device': args.device, 'steps': 1, 'weights_bytes': weights[0].stat().st_size, 'offline': True}), flush=True)
print('OFFLINE_STUDENT_SFT_DPO_OK '+args.device, flush=True)
