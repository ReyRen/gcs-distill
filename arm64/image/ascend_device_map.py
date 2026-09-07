import ctypes
import pathlib
import sys

libraries = (
    '/usr/local/dcmi/libdcmi.so',
    '/usr/local/Ascend/driver/lib64/driver/libdcmi.so',
    '/usr/local/Ascend/driver/lib64/common/libdcmi.so',
)
dcmi = None
errors = []
for candidate in libraries:
    if pathlib.Path(candidate).exists():
        try:
            dcmi = ctypes.CDLL(candidate)
            break
        except OSError as error:
            errors.append(f'{candidate}: {error}')
if dcmi is None:
    try:
        dcmi = ctypes.CDLL('libdcmi.so')
    except OSError as error:
        errors.append(f'libdcmi.so: {error}')
        raise RuntimeError('DCMI library is unavailable: ' + '; '.join(errors))
if dcmi.dcmi_init() != 0:
    raise RuntimeError('DCMI initialization failed; refusing unbound NPU execution')
physical = []
for item in sys.argv[1].split(','):
    item = item.strip()
    if not item.isdecimal():
        raise ValueError(f'invalid physical NPU index: {item!r}')
    value = int(item)
    if value not in physical:
        physical.append(value)
logical = []
for item in physical:
    value = ctypes.c_uint()
    rc = dcmi.dcmi_get_device_logicid_from_phyid(ctypes.c_uint(item), ctypes.byref(value))
    if rc != 0:
        raise RuntimeError(f'DCMI cannot map physical NPU {item}: {rc}')
    if value.value not in logical:
        logical.append(value.value)
if not logical:
    raise RuntimeError('No NPU selected')
print(','.join(str(value) for value in logical))
