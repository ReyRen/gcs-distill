ARG BASE_IMAGE=172.18.127.43:80/sft-images/llama-factory-ascend910b-gcs:0.9.3-cann8-py310-torch2.2.0
FROM ${BASE_IMAGE}

ARG REVISION
ENV PYTHONUNBUFFERED=1 HF_HUB_OFFLINE=1 TRANSFORMERS_OFFLINE=1 HF_DATASETS_OFFLINE=1 \
    HF_HUB_DISABLE_TELEMETRY=1 DO_NOT_TRACK=1 WANDB_DISABLED=true \
    PIP_NO_INDEX=1 PIP_DISABLE_PIP_VERSION_CHECK=1 TOKENIZERS_PARALLELISM=false
COPY source /opt/llamafactory
COPY requirements.lock /opt/llamafactory/gcs-requirements.lock
WORKDIR /opt/llamafactory
RUN --mount=type=bind,source=wheels,target=/wheels \
    python3 -m pip install --no-index --no-deps --find-links=/wheels -r gcs-requirements.lock \
    && python3 -m pip install --no-deps --no-build-isolation . \
    # CANN 8's op-compile-tool metadata lists three Python standard-library modules.
    && (python3 -m pip check > /tmp/pip-check.log || { \
        cat /tmp/pip-check.log; \
        test -z "$(grep -vE '^op-compile-tool 0\.1\.0 requires (getopt|inspect|multiprocessing), which is not installed\.$' /tmp/pip-check.log)" \
        && python3 -c 'import getopt, inspect, multiprocessing'; \
    }) \
    && rm /tmp/pip-check.log \
    && python3 -m pip freeze > /opt/llamafactory/runtime-requirements.lock
COPY --chmod=0755 ascend-train.py /usr/local/bin/gcs-ascend-train
ENV LD_LIBRARY_PATH=/usr/local/Ascend/ascend-toolkit/latest/lib64:${LD_LIBRARY_PATH}

LABEL org.opencontainers.image.title="GCS Distill ARM64 Ascend student trainer" \
      org.opencontainers.image.version="0.9.3-cann8-torch2.2-vl2" \
      org.opencontainers.image.source="https://github.com/hiyouga/LLaMA-Factory" \
      org.opencontainers.image.revision="${REVISION}" \
      gcs.application="gcs-distill" gcs.role="student-trainer"
ENTRYPOINT []
CMD ["llamafactory-cli", "version"]
