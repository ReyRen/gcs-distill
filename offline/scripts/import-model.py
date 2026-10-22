#!/usr/bin/env python3
"""Copy a complete offline model into one user's private student catalog."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--shared-root', type=Path, required=True)
parser.add_argument('--uid', type=int, required=True)
parser.add_argument('--source', type=Path, required=True)
parser.add_argument('--model-id')
args = parser.parse_args()
if args.uid <= 0:
    parser.error('--uid must be positive')
shared = args.shared_root.absolute()
if shared.resolve(strict=True) != shared or not shared.is_dir():
    parser.error('shared root must be an existing real directory')
source = args.source.resolve(strict=True)
model_id = args.model_id or source.name
if not model_id or model_id.startswith('.') or '..' in model_id or any(x in model_id for x in '/\\'):
    parser.error('invalid --model-id')
for name in ['config.json', 'tokenizer_config.json']:
    if not (source / name).is_file():
        parser.error('model is missing ' + name)
files = [path for path in sorted(source.rglob('*'))
         if not any(part.startswith('.') for part in path.relative_to(source).parts)]
if any(path.is_symlink() or not (path.is_dir() or path.is_file()) for path in files):
    parser.error('model must contain real files, without symlinks')
if not any(path.stat().st_size > 0 and (path.suffix == '.safetensors' or
           path.name.startswith('pytorch_model') and path.suffix == '.bin') for path in files if path.is_file()):
    parser.error('model is missing nonempty weights')
for index_name in ['model.safetensors.index.json', 'pytorch_model.bin.index.json']:
    if (source / index_name).exists():
        index = json.loads((source / index_name).read_text())['weight_map']
        if not index or any(Path(name).name != name or not (source / name).is_file() for name in index.values()):
            parser.error('model has missing or invalid weight shards')

base = shared / f'user-{args.uid}/train-center/model-distill/models'
current = shared
for part in base.relative_to(shared).parts:
    current /= part
    current.mkdir(mode=0o750, exist_ok=True)
    if current.is_symlink() or not current.is_dir():
        parser.error('user model directories must not contain symlinks')
for name in ['base', '.imports']:
    path = base / name
    path.mkdir(mode=0o750, exist_ok=True)
    if path.is_symlink() or not path.is_dir():
        parser.error('invalid model import directory')
destination = base / 'base' / model_id
if destination.exists() or destination.is_symlink():
    parser.error('destination already exists; it will not be overwritten')
imports = base / '.imports'
staging = imports / (model_id + '-' + uuid.uuid4().hex)
staging.mkdir(mode=0o750)
expected = {}
if (source / 'MODEL_PROVENANCE.json').is_file():
    provenance = json.loads((source / 'MODEL_PROVENANCE.json').read_text())
    expected = {item['path']: item['sha256'] for item in provenance.get('files', [])}
records = []
try:
    for path in files:
        if not path.is_file() or path.name == 'MODEL_IMPORT.json':
            continue
        relative = path.relative_to(source)
        target = staging / relative
        target.parent.mkdir(mode=0o750, parents=True, exist_ok=True)
        checksum = hashlib.sha256()
        with path.open('rb') as original, target.open('xb') as output:
            for block in iter(lambda: original.read(8 << 20), b''):
                checksum.update(block)
                output.write(block)
        digest = checksum.hexdigest()
        if relative.as_posix() in expected and digest != expected[relative.as_posix()]:
            raise RuntimeError('model SHA256 mismatch: ' + relative.as_posix())
        target.chmod(0o640)
        records.append({'path': relative.as_posix(), 'bytes': target.stat().st_size, 'sha256': digest})
        print('COPIED', model_id, relative.as_posix(), target.stat().st_size, flush=True)
    if not set(expected).issubset({item['path'] for item in records}):
        raise RuntimeError('provenance references missing files')
    manifest = {'uid': args.uid, 'model_id': model_id, 'source': str(source), 'files': records}
    (staging / 'MODEL_IMPORT.json').write_bytes((json.dumps(manifest, indent=2) + '\n').encode('utf-8'))
    os.rename(staging, destination)
except Exception:
    if staging.exists():
        assert staging.resolve().parent == imports.resolve() and not staging.is_symlink()
        shutil.rmtree(staging)
    raise
print('USER_MODEL_IMPORTED', args.uid, destination, sum(x['bytes'] for x in records), flush=True)
