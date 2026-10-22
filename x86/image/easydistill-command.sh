#!/bin/sh
set -eu
# Upstream's default prompt loaders resolve paths relative to its source root.
# Run the unmodified official CLI there; all GCS dataset/config paths are absolute.
cd /opt/easydistill
exec /usr/local/bin/easydistill "$@"
