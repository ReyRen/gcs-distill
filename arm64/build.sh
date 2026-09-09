#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"
out_dir="$script_dir/out"
go_bin="${GO:-go}"
version="${VERSION:-v0.1.0}"
rm -rf "$out_dir"
mkdir -p "$out_dir/bin" "$out_dir/scripts"
cd "$repo_dir"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$go_bin" build -buildvcs=false -trimpath -ldflags "-X main.version=$version" -o "$out_dir/bin/gcs-distill-server" ./cmd/server
cp "$script_dir/config.toml" "$out_dir/config.toml"
cp "$repo_dir/offline/Makefile" "$out_dir/Makefile"
cp "$repo_dir/offline/gcs-distill.service.tpl" "$out_dir/gcs-distill.service.tpl"
cp "$repo_dir/offline/README.md" "$out_dir/README.md"
cp "$repo_dir/offline/PROMPT.md" "$out_dir/PROMPT.md"
cp "$repo_dir/offline/.gitignore" "$out_dir/.gitignore"
cp "$repo_dir/offline/scripts/"*.sh "$out_dir/scripts/"
printf '{\n  "service": "gcs-distill",\n  "architecture": "linux/arm64",\n  "version": "%s",\n  "built_at": "%s"\n}\n' \
  "$version" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$out_dir/BUILD-INFO.json"
echo "ARM64 output: $out_dir"
