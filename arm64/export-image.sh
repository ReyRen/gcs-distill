#!/usr/bin/env bash
set -euo pipefail

image="${1:-gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64}"
output_dir="${2:-/root/images}"
mkdir -p "$output_dir"
safe_name="$(printf '%s' "$image" | tr '/:' '--')"
archive="$output_dir/${safe_name}.tar"
docker save -o "$archive" "$image"
printf '%s\n' "$archive"
