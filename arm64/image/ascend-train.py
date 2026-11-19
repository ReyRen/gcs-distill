#!/usr/bin/env python3
import os
import sys
from pathlib import Path

import torch
import torch_npu


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: gcs-ascend-train /path/to/student_train.json")
    config = str(Path(sys.argv[1]).resolve(strict=True))
    if "LOCAL_RANK" not in os.environ:
        physical = os.environ.get("ASCEND_VISIBLE_DEVICES")
        if not physical:
            raise SystemExit("GCS must assign physical Ascend cards")
        expected = len(physical.split(","))
        count = torch.npu.device_count()
        if count != expected or not torch.npu.is_available():
            raise SystemExit(f"Ascend device assignment expected {expected} devices, found {count}")
        print(f"ASCEND_TRAIN_DEVICES physical={physical} local_count={count}", flush=True)
        if count > 1:
            os.execv(sys.executable, [sys.executable, "-m", "torch.distributed.run", "--standalone",
                                     f"--nproc-per-node={count}", str(Path(__file__).resolve()), config])

    # Keep ephemeral CANN compiler files on the container filesystem. The
    # training config still points to the user's durable JFS paths.
    os.chdir("/tmp")
    torch_npu.npu.set_compile_mode(jit_compile=False)
    from llamafactory.train.tuner import run_exp

    sys.argv[1] = config
    run_exp()


if __name__ == "__main__":
    main()
