#!/bin/sh
set -eu

: "${ASCEND_VISIBLE_DEVICES:?GCS must assign physical Ascend cards}"

# Inside the Ascend container, torch_npu enumerates the selected devices with
# local ranks 0..N-1. Refuse to run if the runtime exposed another device set.
python3 -c '
import os, torch, torch_npu
device_ids = os.environ["ASCEND_VISIBLE_DEVICES"]
selected = device_ids.split(",")
count = torch.npu.device_count()
if count != len(selected) or not torch.npu.is_available():
    raise SystemExit(f"Ascend device assignment expected {len(selected)} devices, found {count}")
print(f"ASCEND_TRAIN_DEVICES physical={device_ids} local_count={count}")
'

exec llamafactory-cli train "$@"
