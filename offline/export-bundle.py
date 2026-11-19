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
p.add_argument('--worker-build-info', required=True, type=Path)
p.add_argument('--data-image', required=True, type=Path)
p.add_argument('--trainer-image', required=True, type=Path)
p.add_argument('--source-info', required=True, type=Path)
p.add_argument('--arch', choices=['amd64', 'arm64'], default='amd64')
a = p.parse_args()
repo = Path(__file__).resolve().parents[1]
root = a.output.resolve()
if root.exists():
    raise RuntimeError('output must be a new directory')
root.mkdir(parents=True)
for n in ['bin', 'images', 'docs', 'locks']:
    (root / n).mkdir()
shutil.copytree(str(repo / 'offline/scripts'), str(root / 'scripts'))
shutil.copytree(str(repo / 'runtime/seed_templates'), str(root / 'dataset-templates'))
readme = 'arm64/README.md' if a.arch == 'arm64' else 'offline/README.md'
shutil.copy2(str(repo / readme), str(root / 'README.md'))
shutil.copy2(str(repo / 'offline/gcs-distill.service.tpl'), str(root / 'gcs-distill.service.tpl'))
config = 'arm64/config.example.toml' if a.arch == 'arm64' else 'config.toml'
shutil.copy2(str(repo / config), str(root / 'config.example.toml'))
docs = ['docs/easydistill2-frontend-handoff.md', 'docs/storage-layout.md', 'server/apidocs/swagger/openapi.json',
        'docs/data-expansion-split-frontend.md', 'docs/generation-performance-frontend.md', 'docs/artifact-quality-frontend.md']
if a.arch == 'amd64':
    docs += ['docs/easydistill2-x86-verification-20261022.md', 'docs/data-preparation-verification-20261105.md',
             'docs/sft-seed-reuse-fix-20261105.md', 'docs/generation-performance-verification-20261109.md',
             'docs/artifact-quality-verification-20261109.md']
for src in docs:
    shutil.copy2(str(repo / src), str(root / 'docs' / Path(src).name))
if a.arch == 'arm64':
    for role in ('data', 'trainer'):
        path = repo / 'arm64/image' / (role + '-requirements.lock')
        shutil.copy2(str(path), str(root / 'locks' / (role + '-build-requirements.lock')))
else:
    shutil.copy2(str(repo / 'x86/image/trainer-constraints.txt'), str(root / 'locks/trainer-constraints.txt'))

def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for b in iter(lambda: f.read(8 << 20), b''):
            h.update(b)
    return h.hexdigest()

worker_build = json.loads(a.worker_build_info.read_text())
if worker_build.get('goos') != 'linux' or worker_build.get('goarch') != a.arch or worker_build.get('binary_sha256') != sha(a.worker):
    raise RuntimeError('worker build metadata does not match the supplied Linux binary')
if not worker_build.get('build_host_glibc') or worker_build.get('cgo_enabled') is not True:
    raise RuntimeError('worker requires verified native glibc and CGO build metadata')
ascend = a.arch == 'arm64'
if ascend and ('ascend' not in worker_build.get('build_tags', []) or not worker_build.get('dcmi_libraries_verified')):
    raise RuntimeError('ARM64 worker needs a native Ascend build and successful DCMI link check')
manifest = {'architecture': 'linux/' + a.arch, 'offline': True, 'images': [], 'binaries': {},
            'sources': json.loads(a.source_info.read_text()),
            'easydistill_revision': 'aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3',
            'trainer_revision': 'ca75f1edf3cb50343ed1c98605141c3e22075b5f',
            'worker_glibc_minimum': worker_build['build_host_glibc'], 'worker_cgo': True, 'worker_ascend': ascend,
            'worker_build': worker_build}
for role, src in [('distill', a.distill), ('manager', a.manager), ('worker', a.worker)]:
    name = {'distill': 'gcs-distill-server', 'manager': 'gcs-V2', 'worker': 'gcs-info-catch-V2'}[role]
    dest = root / 'bin' / name
    shutil.copy2(str(src), str(dest))
    dest.chmod(0o755)
    manifest['binaries'][role] = {'path': 'bin/' + name, 'sha256': sha(dest)}
data_tag = 'gcs/distill-data:2.0.0-aeb8ca3-' + ('arm64' if ascend else 'x86_64')
trainer_tag = 'gcs/distill-trainer:' + ('0.9.3-cann8-torch2.2-vl-arm64' if ascend else '0.9.3-cu124-vl-x86_64')
for role, src, tag in [('data', a.data_image, data_tag), ('trainer', a.trainer_image, trainer_tag)]:
    info = json.loads(subprocess.check_output(['docker', 'image', 'inspect', tag]))[0]
    if info['Os'] + '/' + info['Architecture'] != manifest['architecture']:
        raise RuntimeError('incorrect image architecture')
    suffix = '.tar.gz' if src.name.endswith('.tar.gz') else '.tar'
    dest = root / 'images' / (role + ('-arm64' if ascend else '-x86') + suffix)
    shutil.copy2(str(src), str(dest))
    manifest['images'].append({'role': role, 'tag': tag, 'archive': 'images/' + dest.name,
                               'id': info['Id'], 'sha256': sha(dest), 'bytes': dest.stat().st_size})
    prefix = '/opt/easydistill' if role == 'data' else '/opt/llamafactory'
    docker = ['docker', 'run', '--rm', '--pull=never', '--network', 'none']
    if ascend:
        docker += ['--runtime', 'runc']
    lock = subprocess.check_output(docker + [tag, 'cat', prefix + '/runtime-requirements.lock'])
    (root / 'locks' / (role+'-requirements.lock')).write_bytes(lock)
(root / 'manifest.json').write_bytes((json.dumps(manifest, indent=2) + '\n').encode('utf-8'))
checks = [sha(f) + '  ' + f.relative_to(root).as_posix() for f in sorted(root.rglob('*')) if f.is_file()]
(root / 'SHA256SUMS').write_text('\n'.join(checks) + '\n', encoding='utf-8')
print('OFFLINE_BUNDLE_READY', root)
