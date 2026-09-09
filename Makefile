.PHONY: all build arm64 arm64-clean arm64-image release clean test server swagger run-server tidy fmt lint install deploy start stop restart status logs journal verify uninstall test-integration test-e2e test-all help

BINARY_SERVER ?= bin/gcs-distill-server
VERSION ?= v0.1.0
GO ?= go
SERVICE_NAME ?= gcs-distill

all: build

help:
	@echo "GCS-Distill Makefile commands:"
	@echo "  make build               - update Swagger and build server"
	@echo "  make arm64               - generate arm64/out deployment directory"
	@echo "  make arm64-image         - rebuild the server-side Ascend image"
	@echo "  make release             - build native and ARM64 outputs"
	@echo "  make server              - build server binary"
	@echo "  make swagger             - validate and format OpenAPI"
	@echo "  make test                - run Go tests"
	@echo "  make install             - register this directory with systemd"
	@echo "  make deploy              - build, install, restart and verify"
	@echo "  make status              - show systemd service status"
	@echo "  make logs                - follow service logs"
	@echo "  make verify              - verify systemd and /health"

build: swagger
	@$(MAKE) server SKIP_SWAGGER=1

arm64:
	@bash arm64/build.sh

arm64-clean:
	@rm -rf arm64/out

arm64-image:
	@bash arm64/build-image.sh "$(EASYDISTILL_SOURCE)"

release: build arm64

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

install: build
	$(MAKE) -f offline/Makefile PACKAGE_DIR="$(CURDIR)" UNIT_TEMPLATE="$(CURDIR)/offline/gcs-distill.service.tpl" install

deploy: install
	$(MAKE) -f offline/Makefile PACKAGE_DIR="$(CURDIR)" UNIT_TEMPLATE="$(CURDIR)/offline/gcs-distill.service.tpl" restart
	$(MAKE) -f offline/Makefile PACKAGE_DIR="$(CURDIR)" UNIT_TEMPLATE="$(CURDIR)/offline/gcs-distill.service.tpl" verify

start stop restart status logs journal verify uninstall:
	$(MAKE) -f offline/Makefile PACKAGE_DIR="$(CURDIR)" UNIT_TEMPLATE="$(CURDIR)/offline/gcs-distill.service.tpl" $@

test-integration: test-e2e

test-e2e:
	@bash tests/integration/test_e2e_workflow.sh

test-all: test test-e2e
	@echo "All tests completed."
