#!/usr/bin/env bash
set -euo pipefail
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"
out_dir="$script_dir/out"
go_bin="${GO:-go}"
version="${VERSION:-v0.1.0}"
rm -rf "$out_dir"
mkdir -p "$out_dir/bin" "$out_dir/systemd"
cd "$repo_dir"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 "$go_bin" build -trimpath -ldflags "-X main.version=$version" -o "$out_dir/bin/gcs-distill-server" ./cmd/server
cp "$script_dir/config.toml" "$out_dir/config.toml"
cp "$script_dir/systemd/gcs-distill.service" "$out_dir/systemd/gcs-distill.service"
cp "$script_dir/install.sh" "$out_dir/install.sh"
cp "$script_dir/README.md" "$out_dir/README.md"
echo "ARM64 output: $out_dir"
