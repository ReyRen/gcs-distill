#!/usr/bin/env bash
set -euo pipefail
repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="${1:?Pass a clean official LLaMA-Factory v0.9.3 checkout}"
revision=ca75f1edf3cb50343ed1c98605141c3e22075b5f
[ "$(git -C "$source_dir" rev-parse HEAD)" = "$revision" ]
[ -z "$(git -C "$source_dir" status --porcelain)" ]
context="$(mktemp -d)"
trap 'rm -rf "$context"' EXIT
mkdir -p "$context/trainer" "$context/wheels"
git -C "$source_dir" archive HEAD | tar -x -C "$context/trainer"
cp "$repo_dir/x86/image/trainer.Dockerfile" "$context/Dockerfile"
cp "$repo_dir/x86/image/trainer-constraints.txt" "$context/trainer-constraints.txt"
image=gcs/distill-trainer:0.9.3-cu124-vl-x86_64
docker build --platform linux/amd64 --build-arg TRAINER_REVISION="$revision" -t "$image" "$context"
docker run --rm --network none "$image" llamafactory-cli version
