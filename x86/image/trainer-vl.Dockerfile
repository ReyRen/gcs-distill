ARG BASE_IMAGE=gcs/distill-trainer:0.9.3-cu124-x86_64
FROM ${BASE_IMAGE}
COPY wheels /opt/vision-wheels
RUN python -m pip install --no-index --no-deps /opt/vision-wheels/qwen_vl_utils-0.0.8-py3-none-any.whl \
    && python -m pip check \
    && python -m pip freeze > /opt/llamafactory/runtime-requirements.lock
LABEL org.opencontainers.image.version="0.9.3-torch2.6-cu124-vl1"
