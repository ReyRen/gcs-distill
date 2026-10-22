# x86 image builds

- Data: `gcs/distill-data:2.0.0-aeb8ca3-x86_64`, official EasyDistill revision `aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3`.
- Trainer: `gcs/distill-trainer:0.9.3-cu124-vl-x86_64`, official LLaMA-Factory revision `ca75f1edf3cb50343ed1c98605141c3e22075b5f`.

Build from clean pinned upstream checkouts using `bash x86/build-image.sh SOURCE` and `bash x86/build-trainer.sh SOURCE`. Build hosts may access package indexes; runtime and customer installation never do. Each image records its complete installed dependency set in `runtime-requirements.lock`.

The data wrapper changes working directory to `/opt/easydistill` so upstream relative prompt paths resolve correctly. Official algorithms remain unmodified. GCS controls absolute input/output paths and snapshots teacher settings per run.

Use `docker save` to export both complete images. Load them on each execution node with the offline bundle scripts. Student model weights and tokenizers must already exist in the shared read-only model pool. See [offline deployment](../offline/README.md) and [frontend contract](../docs/easydistill2-frontend-handoff.md).

The trainer includes qwen-vl-utils==0.0.8. For an existing matching CUDA 12.4 image, `image/trainer-vl.Dockerfile` installs a predownloaded wheel with `--network none`; use the complete trainer Dockerfile for a clean build. Record the base image ID and wheel SHA256 when exporting the incremental build.

`prepare-vision-model.py --shared-root /storage-root-jfs` downloads Qwen2-VL-2B-Instruct on an online build host, checks file sizes and upstream SHA256, then atomically publishes the complete model. Existing models are verified rather than replaced. Copy the complete directory and MODEL_PROVENANCE.json to offline sites; runtime never invokes this downloader.
