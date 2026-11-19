ARG BASE_IMAGE=platform_python_arm64_autorun:v1.0
FROM ${BASE_IMAGE}

ARG REVISION
ENV PATH=/opt/gcs/data-venv/bin:${PATH} PYTHONUNBUFFERED=1 \
    HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_DATASETS_OFFLINE=1 \
    HF_HUB_DISABLE_TELEMETRY=1 PIP_NO_INDEX=1 PIP_DISABLE_PIP_VERSION_CHECK=1 DO_NOT_TRACK=1

COPY source /opt/easydistill
COPY requirements.lock /opt/easydistill/gcs-requirements.lock
COPY gcs-easydistill /usr/local/bin/gcs-easydistill
WORKDIR /opt/easydistill
RUN --mount=type=bind,source=wheels,target=/wheels \
    /root/miniconda3/bin/python3 -m venv /opt/gcs/data-venv \
    && python -m pip install --no-index --find-links=/wheels -r gcs-requirements.lock \
    && python -m pip install --no-deps --no-build-isolation . \
    && python -m pip check \
    && python -m pip freeze > runtime-requirements.lock \
    && ln -s /opt/gcs/data-venv/bin/easydistill /usr/local/bin/easydistill \
    && chmod 0755 /usr/local/bin/gcs-easydistill \
    && easydistill --list-jobs > supported-jobs.txt

LABEL org.opencontainers.image.title="GCS EasyDistill 2 ARM64 data runtime" \
      org.opencontainers.image.version="2.0.0" \
      org.opencontainers.image.source="https://github.com/modelscope/easydistill" \
      org.opencontainers.image.revision="${REVISION}" \
      com.gcs.contract="gcs-distill/easydistill2-v2" \
      gcs.application="gcs-distill" gcs.role="data"
ENTRYPOINT []
CMD ["easydistill", "--list-jobs"]
