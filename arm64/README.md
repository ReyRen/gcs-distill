# gcs-distill ARM64 package and EasyDistill image

`make arm64` creates the lightweight service package in `arm64/out/`. It uses the existing MySQL service at `172.18.127.43:3306`.

Build the role-specific image from the same EasyDistill source snapshot used by the x86 deployment:

```bash
bash arm64/build-image.sh /path/to/EasyDistill
```

The resulting image is `gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64`. It keeps the official CANN 9.0/vLLM Ascend inference stack separate from the Transformers 4.x/TRL training environment and exposes all three commands used by gcs-distill:

```text
python -m easydistill.kd.infer
accelerate launch --module easydistill.kd.train
python -m easydistill.eval.data_eval
```

Export the image on the connected build host with `bash arm64/export-image.sh`. The release `offline` directory stores an uncompressed `docker save` archive so onsite staff can use `make load-image`. On the offline Ascend 910B3 host, run the complete image test with a small local model:

```bash
make load-image
bash scripts/test-image.sh /absolute/path/to/Qwen2.5-0.5B-Instruct 0
```

The test performs real local teacher inference, one-process student training, and evaluation through a local OpenAI-compatible mock judge. Its Docker flags and command lines match the GCS worker contract.

Images and archives stay outside this source repository. The 910A deployment on host `172.18.127.43` verifies project, dataset and pipeline CRUD, stage submission, status, logs, cancellation and deletion. A stage may fail after submission when its model or runtime is incompatible with 910A. Real inference, training and evaluation remain the onsite 910B3 image acceptance test.

```bash
make arm64
cd arm64/out
make install
make start
make verify
```

The generated directory may be renamed or moved before installation. systemd
runs the binary and configuration directly from that final directory.
