SHELL := /bin/sh

SERVICE_NAME ?= gcs-distill
UNIT_NAME := $(SERVICE_NAME).service
PACKAGE_DIR := $(realpath $(CURDIR))
SYSTEMD_DIR ?= /etc/systemd/system
UNIT_PATH := $(SYSTEMD_DIR)/$(UNIT_NAME)
BINARY := $(PACKAGE_DIR)/bin/gcs-distill-server
CONFIG := $(PACKAGE_DIR)/config.toml
UNIT_TEMPLATE := $(PACKAGE_DIR)/gcs-distill.service.tpl
VERIFY_TIMEOUT ?= 60
SUDO ?= $(shell if [ "$$(id -u)" -eq 0 ]; then printf ''; else printf sudo; fi)

.PHONY: check install load-image start stop restart status logs journal verify uninstall

check:
	@case "$(PACKAGE_DIR)" in *[[:space:]]*) echo "deployment directory must not contain whitespace: $(PACKAGE_DIR)" >&2; exit 1 ;; esac
	@test -x "$(BINARY)" || { echo "missing executable: $(BINARY)" >&2; exit 1; }
	@test -r "$(CONFIG)" || { echo "missing configuration: $(CONFIG)" >&2; exit 1; }
	@test -r "$(UNIT_TEMPLATE)" || { echo "missing unit template: $(UNIT_TEMPLATE)" >&2; exit 1; }

install: check
	@mkdir -p "$(PACKAGE_DIR)/logs"
	@escaped_dir="$$(printf '%s\n' "$(PACKAGE_DIR)" | sed 's/[&|]/\\&/g')"; sed "s|@@PACKAGE_DIR@@|$$escaped_dir|g" "$(UNIT_TEMPLATE)" | $(SUDO) tee "$(UNIT_PATH)" >/dev/null
	@$(SUDO) systemctl daemon-reload
	@load_state="$$( $(SUDO) systemctl show "$(UNIT_NAME)" --property=LoadState --value )"; test "$$load_state" = loaded || { echo "systemd rejected $(UNIT_PATH): LoadState=$$load_state" >&2; exit 1; }
	@$(SUDO) systemctl enable "$(UNIT_NAME)"
	@printf 'Installed %s with ExecStart=%s\n' "$(UNIT_PATH)" "$(BINARY)"

load-image:
	@bash "$(PACKAGE_DIR)/scripts/load-image.sh"

start: check
	@$(SUDO) systemctl start "$(UNIT_NAME)"

stop:
	@$(SUDO) systemctl stop "$(UNIT_NAME)"

restart: check
	@$(SUDO) systemctl restart "$(UNIT_NAME)"

status:
	@$(SUDO) systemctl status "$(UNIT_NAME)" --no-pager

logs journal:
	@$(SUDO) journalctl -u "$(UNIT_NAME)" -n 200 -f

verify:
	@api_host="$$(awk '/^\[server\][[:space:]]*$$/ { active=1; next } /^\[/ { active=0 } active && /^[[:space:]]*host[[:space:]]*=/ { value=$$0; sub(/^[^=]*=[[:space:]]*/, "", value); gsub(/["[:space:]]/, "", value); print value; exit }' "$(CONFIG)")"; \
		api_port="$$(awk '/^\[server\][[:space:]]*$$/ { active=1; next } /^\[/ { active=0 } active && /^[[:space:]]*port[[:space:]]*=/ { value=$$0; sub(/^[^=]*=[[:space:]]*/, "", value); gsub(/["[:space:]]/, "", value); print value; exit }' "$(CONFIG)")"; \
		test -n "$$api_host" && test -n "$$api_port" || { echo "server.host/server.port is missing from $(CONFIG)" >&2; exit 1; }; \
		[ "$$api_host" != "0.0.0.0" ] || api_host=127.0.0.1; url="http://$$api_host:$$api_port/health"; attempt=0; \
		while [ "$$attempt" -lt "$(VERIFY_TIMEOUT)" ]; do if $(SUDO) systemctl is-active --quiet "$(UNIT_NAME)" && curl --fail --silent "$$url" >/dev/null; then printf 'Verified %s and %s\n' "$(UNIT_NAME)" "$$url"; exit 0; fi; attempt=$$((attempt + 1)); sleep 1; done; \
		echo "verification timed out after $(VERIFY_TIMEOUT)s: $$url" >&2; $(SUDO) systemctl status "$(UNIT_NAME)" --no-pager >&2 || true; exit 1

uninstall:
	@$(SUDO) systemctl disable --now "$(UNIT_NAME)" 2>/dev/null || true
	@$(SUDO) rm -f "$(UNIT_PATH)"
	@$(SUDO) systemctl daemon-reload
	@printf 'Removed %s; package files and logs remain in %s\n' "$(UNIT_PATH)" "$(PACKAGE_DIR)"
