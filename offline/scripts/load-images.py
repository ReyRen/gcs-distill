#!/usr/bin/env python3
import argparse
import hashlib
import json
import subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--bundle', required=True, type=Path)
p.add_argument('--data-only', action='store_true')
a = p.parse_args()
root = a.bundle.resolve()
manifest = json.loads((root / 'manifest.json').read_text())
for item in manifest['images']:
    if a.data_only and item['role'] != 'data':
        continue
    path = (root / item['archive']).resolve()
    if root not in path.parents:
        raise RuntimeError('archive escapes bundle')
    h = hashlib.sha256()
    with path.open('rb') as f:
        for chunk in iter(lambda: f.read(8 << 20), b''):
            h.update(chunk)
    if h.hexdigest() != item['sha256']:
        raise RuntimeError('image checksum mismatch: ' + str(path))
    subprocess.check_call(['docker', 'load', '-i', str(path)])
    actual = json.loads(subprocess.check_output(['docker', 'image', 'inspect', item['tag']]))[0]
    if actual['Id'] != item['id'] or actual['Architecture'] != 'amd64':
        raise RuntimeError('loaded image ID/architecture mismatch')
    print('VERIFIED', item['tag'], actual['Id'])
