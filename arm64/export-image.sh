#!/usr/bin/env bash
set -euo pipefail

image="${1:-gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64}"
output_dir="${2:-/root/images}"
mkdir -p "$output_dir"
safe_name="$(printf '%s' "$image" | tr '/:' '--')"
archive="$output_dir/${safe_name}.tar.gz"

if command -v pigz >/dev/null 2>&1; then
  docker save "$image" | pigz -1 > "$archive"
else
  docker save "$image" | gzip -1 > "$archive"
fi
printf '%s\n' "$archive"
