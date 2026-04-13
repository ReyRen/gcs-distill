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

ARG EASYDISTILL_REPO=https://github.com/modelscope/easydistill.git
ARG EASYDISTILL_REF=main

RUN git clone --depth 1 --branch ${EASYDISTILL_REF} ${EASYDISTILL_REPO} ${EASYDISTILL_HOME}

WORKDIR ${EASYDISTILL_HOME}

RUN python -m pip install --upgrade pip setuptools wheel && \
    pip install -r requirements.txt && \
    pip install .

RUN mkdir -p /workspace/configs /workspace/data /workspace/output /workspace/logs

WORKDIR /workspace

ENTRYPOINT ["easydistill"]
CMD ["--help"]