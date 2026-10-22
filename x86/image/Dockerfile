FROM python@sha256:a630a63cdb314e2d138a2fca3e375e319e8568346ffafac5b980f888630ac4f1
ARG UPSTREAM_REVISION=aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3
ENV PYTHONUNBUFFERED=1 PIP_DISABLE_PIP_VERSION_CHECK=1
COPY upstream /opt/easydistill
WORKDIR /opt/easydistill
RUN python -m pip install --no-cache-dir . \
    && python -m pip freeze > /opt/easydistill/runtime-requirements.lock \
    && easydistill --list-jobs > /opt/easydistill/supported-jobs.txt
ENV HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_DATASETS_OFFLINE=1 \
    HF_HUB_DISABLE_TELEMETRY=1 PIP_NO_INDEX=1 DO_NOT_TRACK=1
LABEL org.opencontainers.image.title="GCS EasyDistill 2 x86 runtime" \
      org.opencontainers.image.version="2.0.0" \
      org.opencontainers.image.source="https://github.com/modelscope/easydistill" \
      org.opencontainers.image.revision="${UPSTREAM_REVISION}" \
      com.gcs.contract="gcs-distill/easydistill2-v2" \
      gcs.application="gcs-distill" gcs.role="data"
COPY easydistill-command.sh /usr/local/bin/gcs-easydistill
RUN chmod 755 /usr/local/bin/gcs-easydistill
ENTRYPOINT []
CMD ["easydistill", "--list-jobs"]
