FROM python@sha256:a630a63cdb314e2d138a2fca3e375e319e8568346ffafac5b980f888630ac4f1
ARG TRAINER_REVISION=ca75f1edf3cb50343ed1c98605141c3e22075b5f
ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
ENV PYTHONUNBUFFERED=1 PIP_DISABLE_PIP_VERSION_CHECK=1
COPY trainer /opt/llamafactory
COPY trainer-constraints.txt /opt/trainer-constraints.txt
WORKDIR /opt/llamafactory
RUN --mount=type=bind,source=wheels,target=/wheels --mount=type=cache,target=/root/.cache/pip if ls /wheels/*.whl >/dev/null 2>&1; then python -m pip install --no-index --no-deps /wheels/*.whl; fi \
    && python -m pip install -c /opt/trainer-constraints.txt --timeout 30 --retries 3 \
      torch==2.6.0 torchvision==0.21.0 \
      transformers==4.52.4 datasets==3.6.0 accelerate==1.7.0 \
      peft==0.15.2 trl==0.9.6 tokenizers==0.21.1 \
      qwen-vl-utils==0.0.8 \
      numpy==1.26.4 pydantic==2.10.6 gradio==5.31.0 imageio-ffmpeg . \
    && python -m pip check \
    && python -m pip freeze > /opt/llamafactory/runtime-requirements.lock
ENV HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_DATASETS_OFFLINE=1 \
    HF_HUB_DISABLE_TELEMETRY=1 DO_NOT_TRACK=1 WANDB_DISABLED=true \
    PIP_NO_INDEX=1 TOKENIZERS_PARALLELISM=false
LABEL org.opencontainers.image.title="GCS Distill offline student trainer" \
      org.opencontainers.image.version="0.9.3-torch2.6-cu124-vl1" \
      org.opencontainers.image.source="https://github.com/hiyouga/LLaMA-Factory" \
      org.opencontainers.image.revision="${TRAINER_REVISION}" \
      gcs.application="gcs-distill" gcs.role="student-trainer"
ENTRYPOINT []
CMD ["llamafactory-cli", "version"]
