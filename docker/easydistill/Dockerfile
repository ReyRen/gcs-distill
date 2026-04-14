ARG BASE_IMAGE=nvidia/cuda:12.1.1-cudnn8-runtime-ubuntu22.04
FROM ${BASE_IMAGE}

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    EASYDISTILL_HOME=/opt/easydistill

RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-dev \
    python3-pip \
    python3-venv \
    git \
    curl \
    ca-certificates \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

RUN ln -sf /usr/bin/python3 /usr/bin/python

WORKDIR /opt

ARG EASYDISTILL_REF=main
ARG EASYDISTILL_ARCHIVE_URL=https://codeload.github.com/modelscope/easydistill/tar.gz/${EASYDISTILL_REF}
ARG PIP_INDEX_URL=
ARG PIP_EXTRA_INDEX_URL=

RUN curl --fail --location --retry 3 --retry-all-errors \
        ${EASYDISTILL_ARCHIVE_URL} \
        --output /tmp/easydistill.tar.gz && \
    archive_root=$(tar -tzf /tmp/easydistill.tar.gz | head -1 | cut -d/ -f1) && \
    tar -xzf /tmp/easydistill.tar.gz -C /opt && \
    mv /opt/${archive_root} ${EASYDISTILL_HOME} && \
    rm -f /tmp/easydistill.tar.gz

WORKDIR ${EASYDISTILL_HOME}

RUN set -eux; \
    python -m pip install --upgrade pip setuptools wheel; \
    pip_args=""; \
    if [ -n "${PIP_INDEX_URL}" ]; then pip_args="$pip_args --index-url ${PIP_INDEX_URL}"; fi; \
    if [ -n "${PIP_EXTRA_INDEX_URL}" ]; then pip_args="$pip_args --extra-index-url ${PIP_EXTRA_INDEX_URL}"; fi; \
    if [ -n "$pip_args" ]; then \
        if python -m pip install $pip_args -r requirements.txt && python -m pip install $pip_args .; then \
            echo "Installed Python dependencies with custom package index"; \
        else \
            echo "Custom package index failed, falling back to default PyPI"; \
            python -m pip install -r requirements.txt; \
            python -m pip install .; \
        fi; \
    else \
        python -m pip install -r requirements.txt; \
        python -m pip install .; \
    fi

RUN mkdir -p /workspace/configs /workspace/data /workspace/output /workspace/logs

WORKDIR /workspace

ENTRYPOINT ["easydistill"]
CMD ["--help"]