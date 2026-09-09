#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
package_dir="$(cd "$script_dir/.." && pwd)"
image="gcs/distill-easydistill-ascend:0.22.1rc1-cann9.0-arm64"
archive="$package_dir/images/gcs-distill-easydistill-ascend-0.22.1rc1-cann9.0-arm64.tar"

command -v docker >/dev/null || { echo 'docker is required' >&2; exit 2; }
test -r "$archive" || { echo "missing image archive: $archive" >&2; exit 2; }
docker load -i "$archive"
docker image inspect "$image" --format 'Loaded {{.RepoTags}} {{.Architecture}} {{.Id}}'
