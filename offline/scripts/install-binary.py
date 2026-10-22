#!/usr/bin/env python3
import argparse
import hashlib
import json
import os
import shutil
import subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--bundle', required=True, type=Path)
p.add_argument('--component', choices=['distill', 'manager', 'worker'], required=True)
p.add_argument('--target', required=True, type=Path)
p.add_argument('--service', required=True)
a = p.parse_args()
root = a.bundle.resolve()
item = json.loads((root / 'manifest.json').read_text())['binaries'][a.component]
src = (root / item['path']).resolve()
if root not in src.parents or hashlib.sha256(src.read_bytes()).hexdigest() != item['sha256']:
    raise RuntimeError('binary checksum mismatch')
if not a.target.is_absolute() or not a.target.is_file() or a.target.is_symlink():
    raise RuntimeError('target must be an existing absolute file; preserve current platform config')
target = a.target.resolve()
if a.component == 'worker':
    linked = subprocess.check_output(['ldd', str(src)], stderr=subprocess.STDOUT).decode()
    if 'not found' in linked:
        raise RuntimeError('worker runtime library missing: ' + linked)
backup = target.with_name(target.name + '.before-distill2')
if not backup.exists():
    shutil.copy2(str(target), str(backup))
pending = target.with_name(target.name + '.distill2-new')
shutil.copy2(str(src), str(pending))
pending.chmod(0o755)
subprocess.check_call(['systemctl', 'stop', a.service])
os.replace(str(pending), str(target))
try:
    subprocess.check_call(['systemctl', 'start', a.service])
    subprocess.check_call(['systemctl', 'is-active', '--quiet', a.service])
except Exception:
    shutil.copy2(str(backup), str(target))
    subprocess.call(['systemctl', 'start', a.service])
    raise
print('INSTALLED', a.component, target, 'configuration preserved')
