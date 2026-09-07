#!/usr/bin/env bash
set -eo pipefail

system_python="$(cat /opt/gcs/system-python)"
clean_path="${PATH//\/opt\/gcs\/bin:/}"
clean_path="${clean_path%:/opt/gcs/bin}"
export PATH="$clean_path"
for setup in /usr/local/Ascend/ascend-toolkit/set_env.sh /usr/local/Ascend/nnal/atb/set_env.sh; do
  if [[ -f "$setup" ]]; then source "$setup"; fi
done
export LD_LIBRARY_PATH="/usr/local/Ascend/driver/lib64/driver:/usr/local/Ascend/driver/lib64/common:${LD_LIBRARY_PATH:-}"

train_python=/opt/gcs/train-venv/bin/python
if [[ -n "${ASCEND_VISIBLE_DEVICES:-}" && "${ASCEND_VISIBLE_DEVICES}" != all ]]; then
  export ASCEND_RT_VISIBLE_DEVICES="$("$system_python" /opt/gcs/ascend_device_map.py "$ASCEND_VISIBLE_DEVICES")"
fi
export PATH="/opt/gcs/bin:$PATH"

command_name="$(basename "$0")"
if [[ "$command_name" == runtime-command ]]; then
  command_name="${1:-contract}"
  if (( $# )); then shift; fi
fi

case "$command_name" in
  contract)
    exec /opt/gcs/bin/gcs-distill-contract "$@"
    ;;
  accelerate)
    exec "$train_python" -m accelerate.commands.accelerate_cli "$@"
    ;;
  python|python3)
    if [[ "${GCS_DISTILL_STAGE:-}" == student_train || " $* " == *" easydistill.kd.train "* ]]; then
      exec "$train_python" "$@"
    fi
    exec "$system_python" "$@"
    ;;
  easydistill)
    exec "$system_python" -m easydistill.cli "$@"
    ;;
  *)
    exec "$command_name" "$@"
    ;;
esac
