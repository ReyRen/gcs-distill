.PHONY: all build clean test server swagger run-server tidy fmt lint install-service enable-service start-service restart-service status-service logs-service deploy test-integration test-e2e test-all help

BINARY_SERVER ?= bin/gcs-distill-server
VERSION ?= v0.1.0
GO ?= go
SUDO ?= sudo
SERVICE_NAME ?= gcs-distill
SERVICE_FILE ?= $(SERVICE_NAME).service
SERVICE_DIR ?= /etc/systemd/system
ENV_DIR ?= /etc/gcs-distill

all: build

help:
	@echo "GCS-Distill Makefile commands:"
	@echo "  make build               - update Swagger and build server"
	@echo "  make server              - build server binary"
	@echo "  make swagger             - validate and format OpenAPI"
	@echo "  make test                - run Go tests"
	@echo "  make deploy              - update Swagger, build binary, install and restart systemd service"
	@echo "  make status-service      - show systemd service status"
	@echo "  make logs-service        - follow systemd service logs"

build: swagger
	@$(MAKE) server SKIP_SWAGGER=1

swagger:
	@echo "Updating Swagger/OpenAPI..."
	@$(GO) run ./cmd/openapi
	@echo "Swagger/OpenAPI updated."

server:
ifeq ($(SKIP_SWAGGER),)
	@$(MAKE) swagger
endif
	@echo "Building server..."
	@mkdir -p bin
	@$(GO) build -o $(BINARY_SERVER) -ldflags "-X main.version=$(VERSION)" ./cmd/server
	@echo "Server built: $(BINARY_SERVER)"

test:
	@echo "Running tests..."
	@$(GO) test -v -race -coverprofile=coverage.out ./...
	@echo "Tests completed."

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out
	@echo "Clean completed."

run-server: server
	@$(BINARY_SERVER) --config config.toml

tidy:
	@$(GO) mod tidy

fmt:
	@gofmt -w .

lint:
	@golangci-lint run ./...

install-service:
	$(SUDO) install -d $(ENV_DIR)
	$(SUDO) install -m 644 $(SERVICE_FILE) $(SERVICE_DIR)/$(SERVICE_FILE)
	$(SUDO) systemctl daemon-reload

enable-service:
	$(SUDO) systemctl enable $(SERVICE_NAME)

start-service:
	$(SUDO) systemctl start $(SERVICE_NAME)

restart-service:
	$(SUDO) systemctl restart $(SERVICE_NAME)

status-service:
	$(SUDO) systemctl status $(SERVICE_NAME)

logs-service:
	$(SUDO) journalctl -u $(SERVICE_NAME) -f

deploy: build install-service enable-service restart-service

test-integration: test-e2e

test-e2e:
	@bash tests/integration/test_e2e_workflow.sh

test-all: test test-e2e
	@echo "All tests completed."
