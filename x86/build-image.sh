#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="${1:?Pass a clean official EasyDistill 2 checkout}"
revision=aeb8ca3eab8b2a82f754c01c3d1f59ad276268c3
image="${IMAGE:-gcs/distill-data:2.0.0-aeb8ca3-x86_64}"
[ "$(git -C "$source_dir" rev-parse HEAD)" = "$revision" ] || { echo 'Unexpected EasyDistill revision' >&2; exit 1; }
[ -z "$(git -C "$source_dir" status --porcelain)" ] || { echo 'EasyDistill source is dirty' >&2; exit 1; }
context="$(mktemp -d)"
trap 'rm -rf "$context"' EXIT
mkdir -p "$context/upstream"
git -C "$source_dir" archive HEAD | tar -x -C "$context/upstream"
cp "$repo_dir/x86/image/Dockerfile" "$context/Dockerfile"
cp "$repo_dir/x86/image/easydistill-command.sh" "$context/easydistill-command.sh"
docker build --platform linux/amd64 --build-arg UPSTREAM_REVISION="$revision" -t "$image" "$context"
docker run --rm --network none "$image" easydistill --list-jobs
docker image inspect "$image" --format '{{.Id}} {{.Architecture}} {{index .Config.Labels "org.opencontainers.image.revision"}}'
