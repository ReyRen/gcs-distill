#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--shared-root', type=Path, required=True)
p.add_argument('--data-only', action='store_true')
accelerator = p.add_mutually_exclusive_group()
accelerator.add_argument('--gpu-device')
accelerator.add_argument('--npu-devices', help='comma-separated physical Ascend device IDs')
p.add_argument('--arch', choices=['amd64', 'arm64'], default='amd64')
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
if a.arch == 'arm64':
    data = 'gcs/distill-data:2.0.0-aeb8ca3-arm64'
    trainer = 'gcs/distill-trainer:0.9.3-cann8-torch2.2-vl-arm64'
else:
    data = 'gcs/distill-data:2.0.0-aeb8ca3-x86_64'
    trainer = 'gcs/distill-trainer:0.9.3-cu124-vl-x86_64'
if (a.gpu_device and a.arch != 'amd64') or (a.npu_devices and a.arch != 'arm64'):
    p.error('accelerator selection must match the target architecture')
docker = ['docker', 'run', '--rm', '--pull=never', '--network', 'none']
cpu = docker + (['--runtime', 'runc'] if a.arch == 'arm64' else [])
tests = [(data, ['easydistill', '--list-jobs'])]
if not a.data_only:
    if a.arch == 'arm64':
        tests.append((trainer, ['python3', '-c', 'from importlib.metadata import version; print(version("llamafactory"), version("torch-npu"))']))
    else:
        tests.append((trainer, ['llamafactory-cli', 'version']))
for image, command in tests:
    platform = subprocess.check_output(['docker', 'image', 'inspect', '--format', '{{.Os}}/{{.Architecture}}', image]).decode().strip()
    if platform != 'linux/' + a.arch:
        raise RuntimeError('target image platform mismatch: ' + image)
    subprocess.check_call(cpu + [image] + command)
if a.gpu_device:
    code = 'import torch; assert torch.cuda.is_available(); x=torch.ones(1,device="cuda"); print(torch.__version__,torch.version.cuda,x.item())'
    subprocess.check_call(docker + ['--gpus', 'device='+a.gpu_device, trainer, 'python', '-c', code])
if a.npu_devices:
    indices = a.npu_devices.split(',')
    if not all(x.isascii() and x.isdigit() for x in indices) or len(set(indices)) != len(indices):
        p.error('provide distinct numeric Ascend card IDs')
    for index in indices:
        board = subprocess.check_output(['npu-smi', 'info', '-t', 'board', '-i', index, '-c', '0'], text=True)
        if not re.search(r'^\s*Chip Name\s*:\s*910B\w*\b', board, re.M | re.I):
            raise RuntimeError('the ARM64 trainer requires Ascend 910B, check card ' + index)
    code = ('import ctypes, torch, torch_npu, llamafactory, qwen_vl_utils; '
            'assert ctypes.CDLL("libopapi.so").aclnnMatmul; '
            'torch_npu.npu.set_compile_mode(jit_compile=False); '
            'assert torch.npu.device_count() == ' + str(len(indices)) + '; '
            'x=torch.ones((1,2,8,8),device="npu:0",dtype=torch.float16,requires_grad=True); '
            'loss=(x@x).sum(); loss.backward(); '
            'assert loss.item()==1024 and x.grad.sum().item()==2048; print("ASCEND_NPU_OK",torch.npu.device_count())')
    subprocess.check_call(docker + ['--runtime', 'ascend', '-e', 'ASCEND_VISIBLE_DEVICES='+a.npu_devices, trainer, 'python3', '-c', code])
if a.model:
    model = a.model.resolve()
    if root not in model.parents:
        raise RuntimeError('model must be under the shared root')
    code = 'from transformers import AutoConfig,AutoTokenizer; AutoConfig.from_pretrained("/model",local_files_only=True); AutoTokenizer.from_pretrained("/model",local_files_only=True); print("OFFLINE_MODEL_OK")'
    subprocess.check_call(cpu + ['-v', str(model)+':/model:ro', trainer, 'python3', '-c', code])
print('PREFLIGHT_OK; verify cross-node shared mounts and teacher reachability separately')
