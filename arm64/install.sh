#!/usr/bin/env bash
set -euo pipefail
package_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
install_dir="${INSTALL_DIR:-/gcs-distill}"
sudo_cmd="${SUDO:-}"; if [ "$(id -u)" -ne 0 ] && [ -z "$sudo_cmd" ]; then sudo_cmd=sudo; fi
[ "$(uname -m)" = aarch64 ] || { echo 'linux/arm64 is required' >&2; exit 1; }
$sudo_cmd install -d -m 755 "$install_dir/bin" "$install_dir/logs"
$sudo_cmd install -m 755 "$package_dir/bin/gcs-distill-server" "$install_dir/bin/gcs-distill-server"
$sudo_cmd install -m 640 "$package_dir/config.toml" "$install_dir/config.toml"
$sudo_cmd install -m 644 "$package_dir/systemd/gcs-distill.service" /etc/systemd/system/gcs-distill.service
$sudo_cmd systemctl daemon-reload
$sudo_cmd systemctl enable gcs-distill.service
$sudo_cmd systemctl restart gcs-distill.service
