import argparse
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import time
import requests

parser = argparse.ArgumentParser(description='Online build-host preparation, never called by offline runtime.')
parser.add_argument('--shared-root', type=Path, default=Path('/storage-root-jfs'))
args = parser.parse_args()
model = 'Qwen/Qwen2-VL-2B-Instruct'
api = 'https://modelscope.cn/api/v1/models/' + model
staging = args.shared_root / '.gcs-distill-imports/Qwen2-VL-2B-Instruct'
destination = args.shared_root / 'train-base-models/Qwen2-VL-2B-Instruct'
if destination.exists():
    manifest = json.loads((destination / 'MODEL_PROVENANCE.json').read_text())
    for item in manifest['files']:
        path = destination / item['path']
        assert path.is_file() and path.stat().st_size == item['bytes']
        checksum = hashlib.sha256()
        with path.open('rb') as stream:
            for block in iter(lambda: stream.read(8 << 20), b''):
                checksum.update(block)
        assert checksum.hexdigest() == item['sha256'], item['path']
    print('VISION_MODEL_ALREADY_VERIFIED', destination, flush=True)
    raise SystemExit(0)
staging.mkdir(parents=True, exist_ok=True)
response = requests.get(api + '/repo/files', params={'Revision': 'master', 'Recursive': 'true'}, timeout=30)
response.raise_for_status()
metadata = response.json()['Data']['Files']
files = [x for x in metadata if x['Path'] != '.gitattributes' and x.get('Type') != 'tree']
assert any(x['Path'] == 'model.safetensors.index.json' for x in files)


def digest(path):
    sha = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(8 << 20), b''):
            sha.update(block)
    return sha.hexdigest()


def download(item):
    name = item['Path']
    assert Path(name).name == name and name not in ('.', '..')
    path = staging / name
    expected = item.get('Sha256') or item.get('SHA256')
    size = item['Size']
    if not (path.is_file() and path.stat().st_size == size and (not expected or digest(path) == expected)):
        part = path.with_name(path.name + '.part')
        for attempt in range(4):
            offset = part.stat().st_size if part.exists() else 0
            try:
                headers = {'Range': f'bytes={offset}-'} if offset else {}
                with requests.get(api + '/repo', params={'Revision': 'master', 'FilePath': name},
                                  headers=headers, stream=True, timeout=(20, 90)) as stream:
                    if stream.status_code not in (200, 206):
                        raise RuntimeError('HTTP ' + str(stream.status_code))
                    append = offset and stream.status_code == 206
                    done = offset if append else 0
                    progress = done
                    with part.open('ab' if append else 'wb') as output:
                        for block in stream.iter_content(4 << 20):
                            output.write(block)
                            done += len(block)
                            if done - progress >= 256 << 20:
                                print('DOWNLOAD', name, done, '/', size, flush=True)
                                progress = done
                assert part.stat().st_size == size, name
                actual = digest(part)
                assert not expected or actual == expected, 'SHA256 mismatch: ' + name
                os.replace(part, path)
                break
            except Exception as error:
                print('RETRY', name, attempt + 1, type(error).__name__, flush=True)
                if attempt == 3:
                    raise
                time.sleep(2)
    actual = digest(path)
    assert path.stat().st_size == size and (not expected or actual == expected)
    print('VERIFIED', name, size, flush=True)
    return {'path': name, 'bytes': size, 'sha256': actual, 'upstream_sha256': expected,
            'revision': item.get('Revision') or item.get('CommitId')}


with concurrent.futures.ThreadPoolExecutor(max_workers=3) as pool:
    verified = list(pool.map(download, files))
index = json.loads((staging / 'model.safetensors.index.json').read_text())
assert all((staging / name).is_file() for name in set(index['weight_map'].values()))
provenance = {'model_id': model, 'source': 'https://modelscope.cn/models/' + model,
              'files': verified, 'license': 'Apache-2.0'}
(staging / 'MODEL_PROVENANCE.json').write_text(json.dumps(provenance, indent=2) + '\n')
assert not destination.exists(), 'Model destination already exists; preserve it'
os.rename(staging, destination)
print('VISION_MODEL_READY', destination, sum(x['bytes'] for x in verified), flush=True)
