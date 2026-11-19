ARG BASE_IMAGE=quay.io/ascend/cann:9.0.0-910b-ubuntu22.04-py3.11
FROM ${BASE_IMAGE}

ARG REVISION
ENV VIRTUAL_ENV=/opt/gcs/trainer-venv
# Keep CANN's Protobuf version and drop the SDK's optional analysis tools.
RUN python3 -m pip uninstall --yes --root-user-action=ignore ms-service-profiler mindstudio-kpp \
    && python3 -m venv --system-site-packages "$VIRTUAL_ENV" \
    && "$VIRTUAL_ENV/bin/python3" -m pip uninstall --yes setuptools
ENV PATH=${VIRTUAL_ENV}/bin:${PATH} \
    LD_LIBRARY_PATH=/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common:/usr/local/Ascend/driver/lib64:${LD_LIBRARY_PATH} \
    PYTHONUNBUFFERED=1 HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_DATASETS_OFFLINE=1 \
    HF_HUB_DISABLE_TELEMETRY=1 DO_NOT_TRACK=1 WANDB_DISABLED=true \
    PIP_NO_INDEX=1 PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1 TOKENIZERS_PARALLELISM=false

WORKDIR /opt/llamafactory
COPY requirements.lock gcs-requirements.lock
RUN --mount=type=bind,source=wheels,target=/wheels \
    --mount=type=bind,source=source,target=/source,rw \
    python3 -m pip install --no-index --ignore-installed --no-deps --no-compile --require-hashes --find-links=/wheels -r gcs-requirements.lock \
    && python3 -m pip install --no-index --no-deps --no-compile --no-build-isolation /source \
    && python3 -m pip check \
    && python3 -m pip list --local --format=freeze > runtime-requirements.lock
COPY --chmod=0755 ascend-train.py /usr/local/bin/gcs-ascend-train

LABEL org.opencontainers.image.title="GCS Distill ARM64 Ascend 910B student trainer" \
      org.opencontainers.image.version="0.9.3-cann9.0.0-torch2.10.0-hdk25.5.1-vl-arm64" \
      org.opencontainers.image.source="https://github.com/hiyouga/LLaMA-Factory" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.base.name="quay.io/ascend/cann:9.0.0-910b-ubuntu22.04-py3.11" \
      org.opencontainers.image.base.digest="sha256:5d90a091e383f46cfe81c1e4589d23326e6baea5d475a6509a6d600d2520019d" \
      gcs.application="gcs-distill" gcs.role="student-trainer" gcs.ascend.hdk="25.5.1"
ENTRYPOINT []
CMD ["llamafactory-cli", "version"]
