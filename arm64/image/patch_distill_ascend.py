"""Small hardware adapter over the same EasyDistill source used by GCS."""
from pathlib import Path

root = Path('/opt/gcs/easydistill/easydistill')
for name in ('kd/infer.py', 'kd/train.py'):
    path = root / name
    source = path.read_text()
    if 'import torch_npu' not in source:
        if 'import torch\n' not in source:
            raise RuntimeError(f'cannot find torch import in {path}')
        source = source.replace('import torch\n', 'import torch\nimport torch_npu\n', 1)
    source = source.replace('torch.cuda.device_count()', 'torch.npu.device_count()')
    path.write_text(source)

infer = (root / 'kd/infer.py').read_text()
if 'torch.cuda.device_count()' in infer or 'torch.npu.device_count()' not in infer:
    raise RuntimeError('EasyDistill inference NPU patch was not applied')
