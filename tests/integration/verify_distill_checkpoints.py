import argparse
import json
import math
from pathlib import Path
import torch
from safetensors import safe_open

torch.set_num_threads(2)
parser = argparse.ArgumentParser(description='Validate actual optimizer steps and finite LoRA checkpoints on CPU.')
parser.add_argument('inputs', type=Path, nargs='+')
parser.add_argument('--report', type=Path, required=True)
args = parser.parse_args()
reports = []
for path in args.inputs:
    source = json.loads(path.read_text())
    assert source['result'] == 'passed', source
    stage = next(x for x in source['stages'] if x['stage'] == 'student_train')
    checkpoint = Path(stage['output']['checkpoint_path'])
    state = json.loads((checkpoint / 'trainer_state.json').read_text())
    metrics = json.loads((checkpoint / 'train_results.json').read_text())
    assert state['global_step'] > 0 and math.isfinite(metrics['train_loss']) and metrics['train_loss'] > 0
    nonzero_b = 0
    tensors = 0
    with safe_open(checkpoint / 'adapter_model.safetensors', framework='pt', device='cpu') as weights:
        for key in weights.keys():
            tensor = weights.get_tensor(key)
            assert torch.isfinite(tensor).all().item(), key
            tensors += 1
            if 'lora_B' in key:
                nonzero_b += torch.count_nonzero(tensor).item()
    assert tensors > 0 and nonzero_b > 0, 'Adapter must contain actual optimizer updates'
    item = {'mode': source['mode'], 'student': source['student'], 'global_step': state['global_step'],
            'train_loss': metrics['train_loss'], 'finite_adapter_tensors': tensors, 'nonzero_lora_b_values': nonzero_b,
            'result': 'passed'}
    reports.append(item)
    print(json.dumps(item), flush=True)
args.report.write_text(json.dumps(reports, indent=2))
