#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
role="${1:?usage: build-image.sh data|trainer|trainer-hdk25.5.1 source.tar wheelhouse}"
source_tar="${2:?source must be a git archive in tar format}"
wheels="${3:?provide the predownloaded ARM64 wheels}"

case "$role" in
  data)
    revision=aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3
    tag=gcs/distill-data:2.0.0-aeb8ca3-arm64
    base=platform_python_arm64_autorun:v1.0
    base_id=ae2a214c776f38b4b0756fdbbf0478e0de865c65d1ea078018cbf1c7374b0a6a
    check=(easydistill --list-jobs)
    ;;
  trainer)
    revision=ca75f1edf3cb50343ed1c98605141c3e22075b5f
    tag=gcs/distill-trainer:0.9.3-cann8-torch2.2-vl-arm64
    base=172.18.127.43:80/sft-images/llama-factory-ascend910b-gcs:0.9.3-cann8-py310-torch2.2.0
    base_id=151b6854f3062c22ff1d60d5f9a26e302b96af3446807bfdc95b336882260542
    check=(python3 -c 'from importlib.metadata import version; assert version("llamafactory") == "0.9.3"; assert version("torch-npu") == "2.2.0"; assert version("qwen-vl-utils") == "0.0.8"')
    ;;
  trainer-hdk25.5.1)
    revision=ca75f1edf3cb50343ed1c98605141c3e22075b5f
    tag=gcs/distill-trainer:0.9.3-cann9.0.0-torch2.10.0-hdk25.5.1-vl-arm64
    base=quay.io/ascend/cann:9.0.0-910b-ubuntu22.04-py3.11
    base_id=d24afa915a3e423bee809e615e527781b336261f7940581ffcae42ae1724ca0b
    check=(python3 -c 'from importlib.metadata import version; assert version("llamafactory") == "0.9.3"; assert version("torch") == "2.10.0+cpu"; assert version("torch-npu") == "2.10.0"; assert version("qwen-vl-utils") == "0.0.8"')
    ;;
  *) echo "unknown image role: $role" >&2; exit 2 ;;
esac

test "$(uname -s)/$(uname -m)" = Linux/aarch64
test "$(git get-tar-commit-id < "$source_tar")" = "$revision"
test "$(docker image inspect --format '{{.Os}}/{{.Architecture}} {{.Id}}' "$base")" = "linux/arm64 sha256:$base_id"
test -d "$wheels"

context="$(mktemp -d)"
trap 'rm -rf -- "$context"' EXIT
mkdir "$context/source"
tar -xf "$source_tar" -C "$context/source"
cp -R "$wheels" "$context/wheels"
cp "$repo/arm64/image/$role.Dockerfile" "$context/Dockerfile"
cp "$repo/arm64/image/$role-requirements.lock" "$context/requirements.lock"
if [ "$role" = data ]; then
  cp "$repo/x86/image/easydistill-command.sh" "$context/gcs-easydistill"
else
  cp "$repo/arm64/image/ascend-train.py" "$context/"
fi
docker build --platform linux/arm64 --pull=false --network none \
  --build-arg BASE_IMAGE="$base" --build-arg REVISION="$revision" -t "$tag" "$context"
docker run --rm --pull=never --runtime runc --network none "$tag" "${check[@]}"
docker image inspect --format '{{.Os}}/{{.Architecture}} {{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}' "$tag"
