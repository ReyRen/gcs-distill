FROM quay.io/ascend/vllm-ascend@sha256:9008b47081282612abfe4d28069ce34436752c980fd06f7599343213205ce64d

USER root
ARG BUILD_DATE=unknown
ARG VCS_REF=unknown
ARG EASYDISTILL_SOURCE_SHA256=unknown

ENV PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple \
    PIP_DEFAULT_TIMEOUT=120 \
    PIP_RETRIES=3 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

# Teacher inference keeps the official vLLM environment. Training gets a
# separate venv because EasyDistill/TRL require Transformers 4.x while vLLM
# Ascend 0.22.1 uses Transformers 5.x.
RUN python3 -m pip install --no-cache-dir 'jsonlines==4.0.0' 'openai==2.31.0' \
    && python3 -m venv --system-site-packages /opt/gcs/train-venv \
    && /opt/gcs/train-venv/bin/python -m pip install --no-cache-dir \
      'numpy==1.26.4' \
      'transformers==4.51.1' \
      'transformers-stream-generator==0.0.5' \
      'tokenizers==0.21.1' \
      'trl==0.17.0' \
      'accelerate==1.13.0' \
      'datasets==4.8.4' \
      'jsonlines==4.0.0'
RUN command -v python3 > /opt/gcs/system-python

COPY easydistill /opt/gcs/easydistill
COPY patch_distill_ascend.py /opt/gcs/patch_distill_ascend.py
RUN python3 /opt/gcs/patch_distill_ascend.py \
    && python3 -m pip install --no-deps --no-build-isolation /opt/gcs/easydistill \
    && /opt/gcs/train-venv/bin/python -m pip install --no-deps --no-build-isolation /opt/gcs/easydistill \
    && python3 -m pip freeze > /opt/gcs/requirements-infer.lock \
    && /opt/gcs/train-venv/bin/python -m pip freeze > /opt/gcs/requirements-train.lock

COPY distill-command.sh /opt/gcs/bin/runtime-command
COPY ascend_device_map.py /opt/gcs/ascend_device_map.py
COPY gcs-distill-contract.py /opt/gcs/bin/gcs-distill-contract
RUN chmod 755 /opt/gcs/bin/runtime-command /opt/gcs/bin/gcs-distill-contract \
    && for cmd in python python3 accelerate; do ln -s runtime-command "/opt/gcs/bin/$cmd"; done \
    && ln -s /opt/gcs/bin/gcs-distill-contract /usr/local/bin/gcs-distill-contract

LABEL org.opencontainers.image.title="GCS EasyDistill Ascend runtime" \
      org.opencontainers.image.description="ARM64 EasyDistill inference, training and evaluation runtime for gcs-distill" \
      org.opencontainers.image.version="vllm-ascend-0.22.1rc1-cann-9.0.0" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      com.gcs.contract="gcs-distill/stages-v1" \
      com.gcs.target="Ascend-910B3-64GB-HDK-25.3.x" \
      com.gcs.easydistill-source-sha256="${EASYDISTILL_SOURCE_SHA256}"

ENV PATH="/opt/gcs/bin:${PATH}" \
    PYTHONPATH=/opt/gcs/easydistill \
    PYTHONNOUSERSITE=1 \
    HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1 \
    HF_DATASETS_OFFLINE=1 \
    TOKENIZERS_PARALLELISM=false \
    WANDB_DISABLED=true \
    VLLM_WORKER_MULTIPROC_METHOD=spawn

ENTRYPOINT ["/opt/gcs/bin/runtime-command"]
CMD ["contract"]
