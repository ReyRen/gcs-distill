#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
easydistill_source="${1:-${EASYDISTILL_SOURCE:-}}"
[ -n "$easydistill_source" ] && [ -f "$easydistill_source/setup.py" ] || {
  echo 'Pass an EasyDistill source checkout as the first argument.' >&2
  exit 2
}
image="${IMAGE:-gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64}"
build_date="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
vcs_ref="${VCS_REF:-$(git -C "$script_dir/.." rev-parse --short HEAD 2>/dev/null || echo unknown)}"
source_sha256="$(find "$easydistill_source" -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')"
context="$(mktemp -d)"
trap 'rm -rf "$context"' EXIT
cp -a "$easydistill_source" "$context/easydistill"
rm -rf "$context/easydistill/.git" "$context/easydistill/build" \
  "$context/easydistill"/*.egg-info
cp -a "$script_dir/image/." "$context/"
docker build \
  --platform linux/arm64 \
  --build-arg BUILD_DATE="$build_date" \
  --build-arg VCS_REF="$vcs_ref" \
  --build-arg EASYDISTILL_SOURCE_SHA256="$source_sha256" \
  --tag "$image" \
  "$context"
docker image inspect "$image" --format '{{.Id}} {{.Architecture}} {{index .Config.Labels "com.gcs.contract"}}'
docker run --rm --entrypoint gcs-distill-contract "$image"
printf 'Built %s from EasyDistill source %s\n' "$image" "$source_sha256"
