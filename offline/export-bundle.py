#!/usr/bin/env python3
"""Package already-built artifacts. This script never fetches dependencies."""
import argparse
import hashlib
import json
import shutil
import subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--output', required=True, type=Path)
p.add_argument('--distill', required=True, type=Path)
p.add_argument('--manager', required=True, type=Path)
p.add_argument('--worker', required=True, type=Path)
p.add_argument('--data-image', required=True, type=Path)
p.add_argument('--trainer-image', required=True, type=Path)
p.add_argument('--source-info', required=True, type=Path)
a = p.parse_args()
repo = Path(__file__).resolve().parents[1]
root = a.output.resolve()
if root.exists():
    raise RuntimeError('output must be a new directory')
root.mkdir(parents=True)
for n in ['bin', 'images', 'docs', 'locks', 'tests']:
    (root / n).mkdir()
shutil.copytree(str(repo / 'offline/scripts'), str(root / 'scripts'))
shutil.copy2(str(repo / 'offline/README.md'), str(root / 'README.md'))
shutil.copy2(str(repo / 'offline/gcs-distill.service.tpl'), str(root / 'gcs-distill.service.tpl'))
shutil.copy2(str(repo / 'config.toml'), str(root / 'config.example.toml'))
for src in ['docs/easydistill2-frontend-handoff.md', 'docs/easydistill2-x86-verification-20261022.md', 'docs/storage-layout.md', 'server/apidocs/swagger/openapi.json']:
    shutil.copy2(str(repo / src), str(root / 'docs' / Path(src).name))
for src in (repo / 'tests/integration').glob('*.py'):
    shutil.copy2(str(src),str(root / 'tests' / src.name))
shutil.copy2(str(repo / 'tests/integration/README.md'), str(root / 'tests/README.md'))
shutil.copy2(str(repo / 'x86/image/trainer-constraints.txt'),str(root / 'locks/trainer-constraints.txt'))

def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for b in iter(lambda: f.read(8 << 20), b''):
            h.update(b)
    return h.hexdigest()

manifest = {'architecture': 'linux/amd64', 'offline': True, 'images': [], 'binaries': {},
            'sources': json.loads(a.source_info.read_text()),
            'easydistill_revision': 'aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3',
            'trainer_revision': 'ca75f1edf3cb50343ed1c98605141c3e22075b5f',
            'worker_glibc_minimum': '2.31', 'worker_cgo': True, 'worker_ascend': False}
for role, src in [('distill', a.distill), ('manager', a.manager), ('worker', a.worker)]:
    name = {'distill': 'gcs-distill-server', 'manager': 'gcs-V2', 'worker': 'gcs-info-catch-V2'}[role]
    dest = root / 'bin' / name
    shutil.copy2(str(src), str(dest))
    dest.chmod(0o755)
    manifest['binaries'][role] = {'path': 'bin/' + name, 'sha256': sha(dest)}
for role, src, tag in [('data', a.data_image, 'gcs/distill-data:2.0.0-aeb8ca3-x86_64'),
                       ('trainer', a.trainer_image, 'gcs/distill-trainer:0.9.3-cu124-vl-x86_64')]:
    info = json.loads(subprocess.check_output(['docker', 'image', 'inspect', tag]))[0]
    if info['Architecture'] != 'amd64':
        raise RuntimeError('incorrect image architecture')
    dest = root / 'images' / (role + '-x86.tar')
    shutil.copy2(str(src), str(dest))
    manifest['images'].append({'role': role, 'tag': tag, 'archive': 'images/' + dest.name,
                               'id': info['Id'], 'sha256': sha(dest), 'bytes': dest.stat().st_size})
    prefix = '/opt/easydistill' if role == 'data' else '/opt/llamafactory'
    lock = subprocess.check_output(['docker','run','--rm','--pull=never','--network','none',tag,'cat',prefix+'/runtime-requirements.lock'])
    (root / 'locks' / (role+'-requirements.lock')).write_bytes(lock)
(root / 'manifest.json').write_bytes((json.dumps(manifest, indent=2) + '\n').encode('utf-8'))
checks = [sha(f) + '  ' + f.relative_to(root).as_posix() for f in sorted(root.rglob('*')) if f.is_file()]
(root / 'SHA256SUMS').write_bytes(('\n'.join(checks) + '\n').encode('ascii'))
print('OFFLINE_BUNDLE_READY', root)
