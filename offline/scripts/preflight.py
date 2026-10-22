#!/usr/bin/env python3
import argparse
import json
import os
import subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--shared-root', type=Path, required=True)
p.add_argument('--data-only', action='store_true')
p.add_argument('--gpu-device')
p.add_argument('--model', type=Path)
p.add_argument('--container-network', help='optional existing worker network name or ID')
a = p.parse_args()
root = a.shared_root.resolve()
if not root.is_dir() or not os.path.ismount(str(root)):
    raise RuntimeError('shared storage root must already be mounted')
subprocess.check_call(['docker', 'info', '--format', '{{.OSType}}/{{.Architecture}}'])
if a.container_network:
    net = json.loads(subprocess.check_output(['docker', 'network', 'inspect', a.container_network]))[0]
    if net.get('Driver') == 'overlay' and not net.get('Attachable'):
        raise RuntimeError('worker overlay network must allow containers to attach')
data = 'gcs/distill-data:2.0.0-aeb8ca3-x86_64'
trainer = 'gcs/distill-trainer:0.9.3-cu124-vl-x86_64'
tests = [(data, ['easydistill', '--list-jobs'])]
if not a.data_only:
    tests.append((trainer, ['llamafactory-cli', 'version']))
for image, command in tests:
    subprocess.check_call(['docker', 'run', '--rm', '--pull=never', '--network', 'none', image] + command)
if a.gpu_device:
    code = 'import torch; assert torch.cuda.is_available(); x=torch.ones(1,device="cuda"); print(torch.__version__,torch.version.cuda,x.item())'
    subprocess.check_call(['docker', 'run', '--rm', '--pull=never', '--network', 'none', '--gpus', 'device='+a.gpu_device, trainer, 'python', '-c', code])
if a.model:
    model = a.model.resolve()
    if root not in model.parents:
        raise RuntimeError('model must be under the shared root')
    code = 'from transformers import AutoConfig,AutoTokenizer; AutoConfig.from_pretrained("/model",local_files_only=True); AutoTokenizer.from_pretrained("/model",local_files_only=True); print("OFFLINE_MODEL_OK")'
    subprocess.check_call(['docker', 'run', '--rm', '--pull=never', '--network', 'none', '-v', str(model)+':/model:ro', trainer, 'python', '-c', code])
print('PREFLIGHT_OK; verify cross-node shared mounts and teacher reachability separately')
