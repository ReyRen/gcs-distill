.PHONY: all build clean test server swagger docker-build docker-test docker-up docker-down docker-logs docker-restart docker-build-all docker-prune docker-builder-prune run-server tidy fmt lint db-init test-integration test-e2e test-all help

BINARY_SERVER=bin/gcs-distill-server
VERSION=v0.1.0
DOCKER_IMAGE=gcs-distill/easydistill
DOCKER_TAG=latest
COMPOSE=docker compose
PIP_INDEX_URL?=
PIP_EXTRA_INDEX_URL?=
DB_HOST?=127.0.0.1
DB_PORT?=3306
DB_NAME?=ai_market
DB_USER?=root

all: build

help:
	@echo "GCS-Distill Makefile commands:"
	@echo "  make build          - update Swagger and build server"
	@echo "  make server         - build server binary"
	@echo "  make swagger        - validate and format OpenAPI"
	@echo "  make test           - run Go tests"
	@echo "  make docker-build   - build EasyDistill runtime image"
	@echo "  make docker-up      - start distill server, using shared MySQL/GCS from config.toml"
	@echo "  make db-init        - apply MySQL schema to the shared ai_market database"

build: swagger
	@$(MAKE) server SKIP_SWAGGER=1

swagger:
	@echo "Updating Swagger/OpenAPI..."
	@go run ./cmd/openapi
	@echo "Swagger/OpenAPI updated."

server:
ifeq ($(SKIP_SWAGGER),)
	@$(MAKE) swagger
endif
	@echo "Building server..."
	@mkdir -p bin
	@go build -o $(BINARY_SERVER) -ldflags "-X main.version=$(VERSION)" ./cmd/server
	@echo "Server built: $(BINARY_SERVER)"

test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests completed."

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out
	@echo "Clean completed."

docker-build:
	@echo "Building EasyDistill runtime image..."
	@docker build \
		$(if $(PIP_INDEX_URL),--build-arg PIP_INDEX_URL=$(PIP_INDEX_URL)) \
		$(if $(PIP_EXTRA_INDEX_URL),--build-arg PIP_EXTRA_INDEX_URL=$(PIP_EXTRA_INDEX_URL)) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) -f docker/easydistill/Dockerfile .
	@echo "Image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

docker-test:
	@docker run --rm $(DOCKER_IMAGE):$(DOCKER_TAG) --help

run-server: server
	@$(BINARY_SERVER) --config config.toml

tidy:
	@go mod tidy

fmt:
	@gofmt -w .

lint:
	@golangci-lint run ./...

db-init:
	@mysql -h $(DB_HOST) -P $(DB_PORT) -u $(DB_USER) -p $(DB_NAME) < migrations/001_distill_mysql.sql

docker-up:
	@$(COMPOSE) build --no-cache
	@$(COMPOSE) up -d
	@$(COMPOSE) ps

docker-down:
	@$(COMPOSE) down

docker-logs:
	@$(COMPOSE) logs -f

docker-build-all:
	@$(COMPOSE) build gcs-server
	@$(MAKE) docker-build

docker-prune:
	@docker image prune -f

docker-builder-prune:
	@docker builder prune -f

docker-restart:
	@$(COMPOSE) restart
	@$(COMPOSE) ps

test-integration:
	@bash tests/integration/test_easydistill.sh

test-e2e:
	@bash tests/integration/test_e2e_workflow.sh

test-all: test test-integration test-e2e
	@echo "All tests completed."
