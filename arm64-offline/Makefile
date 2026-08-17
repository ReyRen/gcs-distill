SERVICE_NAME := gcs-distill
SERVICE_FILE := $(SERVICE_NAME).service
BINARY := gcs-distill-server
INSTALL_DIR ?= /opt/gcs/gcs-distill
SERVICE_DIR ?= /etc/systemd/system
SUDO ?=

.PHONY: verify install install-config enable start stop restart status logs deploy uninstall

verify:
	@test "$$(uname -m)" = "aarch64" || { echo "ERROR: only linux/arm64 (aarch64) is supported"; exit 1; }
	@test -f $(BINARY) || { echo "ERROR: missing binary $(BINARY)"; exit 1; }
	@sha256sum -c CHECKSUMS.sha256

install: verify
	$(SUDO) install -d -m 755 $(INSTALL_DIR)
	$(SUDO) install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	$(MAKE) install-config
	$(SUDO) install -m 644 $(SERVICE_FILE) $(SERVICE_DIR)/$(SERVICE_FILE)
	$(SUDO) systemctl daemon-reload

install-config:
	$(SUDO) install -m 640 config.toml $(INSTALL_DIR)/config.toml

enable:
	$(SUDO) systemctl enable $(SERVICE_NAME)

start:
	$(SUDO) systemctl start $(SERVICE_NAME)

stop:
	$(SUDO) systemctl stop $(SERVICE_NAME)

restart:
	$(SUDO) systemctl restart $(SERVICE_NAME)

status:
	$(SUDO) systemctl status $(SERVICE_NAME) --no-pager -l

logs:
	$(SUDO) journalctl -u $(SERVICE_NAME) -f

deploy: install enable restart

uninstall:
	$(SUDO) systemctl disable --now $(SERVICE_NAME) 2>/dev/null || true
	$(SUDO) rm -f $(SERVICE_DIR)/$(SERVICE_FILE)
	$(SUDO) systemctl daemon-reload
	@echo "Application files are preserved in $(INSTALL_DIR)."
