.PHONY: all build clean test proto server worker docker-build help

# 变量定义
BINARY_SERVER=bin/gcs-distill-server
BINARY_WORKER=bin/gcs-distill-worker
VERSION=v0.1.0
DOCKER_IMAGE=gcs-distill/easydistill
DOCKER_TAG=latest
COMPOSE=docker compose
PIP_INDEX_URL?=
PIP_EXTRA_INDEX_URL?=

# 默认目标
all: build

## help: 显示帮助信息
help:
	@echo "GCS-Distill Makefile 命令:"
	@echo ""
	@echo "本地编译检查:"
	@echo "  make build          - 编译服务端和 Worker"
	@echo "  make server         - 仅编译服务端"
	@echo "  make worker         - 仅编译 Worker"
	@echo ""
	@echo "Docker 环境:"
	@echo "  make docker-up      - 构建并启动 Docker Compose 环境"
	@echo "  make docker-up-clean - 清理缓存并重新构建启动 Docker Compose 环境"
	@echo "  make docker-up-server - 重建并启动 gcs-server"
	@echo "  make docker-up-server-clean - 清理缓存并重建启动 gcs-server"
	@echo "  make docker-up-worker - 重建并启动 gcs-worker-1"
	@echo "  make docker-down    - 停止 Docker Compose 环境"
	@echo "  make docker-restart - 重启已有 Docker Compose 容器"
	@echo "  make docker-logs    - 查看 Docker Compose 日志"
	@echo "  make docker-build   - 构建 EasyDistill Docker 镜像"
	@echo "  make docker-test    - 测试 EasyDistill Docker 镜像"
	@echo "  make docker-build-all - 构建所有 Docker 镜像"
	@echo "  make docker-prune   - 清理 <none> 悬空镜像"
	@echo "  make docker-builder-prune - 清理 Docker 构建缓存"
	@echo ""
	@echo "测试和验证:"
	@echo "  make test           - 运行 Go 单元测试"
	@echo "  make test-integration - 运行集成测试"
	@echo "  make test-e2e       - 运行端到端测试"
	@echo "  make test-all       - 运行所有测试"
	@echo ""
	@echo "开发工具:"
	@echo "  make proto          - 生成 gRPC 代码"
	@echo "  make clean          - 清理编译产物"
	@echo "  make fmt            - 格式化代码"
	@echo "  make tidy           - 整理 Go 依赖"
	@echo "  make help           - 显示此帮助信息"
	@echo ""

## build: 编译服务端和 Worker
build: server worker

## server: 编译服务端
server:
	@echo "编译服务端..."
	@mkdir -p bin
	@go build -o $(BINARY_SERVER) -ldflags "-X main.version=$(VERSION)" ./cmd/server
	@echo "服务端编译完成: $(BINARY_SERVER)"

## worker: 编译 Worker
worker:
	@echo "编译 Worker..."
	@mkdir -p bin
	@go build -o $(BINARY_WORKER) -ldflags "-X main.version=$(VERSION)" ./cmd/worker
	@echo "Worker 编译完成: $(BINARY_WORKER)"

## proto: 生成 gRPC 代码
proto:
	@echo "生成 gRPC 代码..."
	@PATH="$(shell go env GOPATH)/bin:$$PATH" protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. proto/worker.proto
	@echo "gRPC 代码生成完成"

## test: 运行测试
test:
	@echo "运行测试..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo "测试完成"

## clean: 清理编译产物
clean:
	@echo "清理编译产物..."
	@rm -rf bin/
	@rm -f coverage.out
	@echo "清理完成"

## docker-build: 构建 EasyDistill Docker 镜像
docker-build:
	@echo "构建 EasyDistill Docker 镜像..."
	@docker build \
		$(if $(PIP_INDEX_URL),--build-arg PIP_INDEX_URL=$(PIP_INDEX_URL)) \
		$(if $(PIP_EXTRA_INDEX_URL),--build-arg PIP_EXTRA_INDEX_URL=$(PIP_EXTRA_INDEX_URL)) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) -f docker/easydistill/Dockerfile .
	@echo "镜像构建完成: $(DOCKER_IMAGE):$(DOCKER_TAG)"

## docker-test: 测试 EasyDistill Docker 镜像
docker-test:
	@echo "测试 EasyDistill Docker 镜像..."
	@docker run --rm $(DOCKER_IMAGE):$(DOCKER_TAG) --help
	@echo "镜像测试完成"

## run-server: 运行服务端 (开发模式)
run-server: server
	@echo "启动服务端..."
	@$(BINARY_SERVER) --config config.example.yaml

## run-worker: 运行 Worker (开发模式)
run-worker: worker
	@echo "启动 Worker..."
	@$(BINARY_WORKER) --config config.example.yaml --node-name worker-dev-1

## tidy: 整理 Go 依赖
tidy:
	@echo "整理 Go 依赖..."
	@go mod tidy
	@echo "依赖整理完成"

## fmt: 格式化代码
fmt:
	@echo "格式化代码..."
	@go fmt ./...
	@echo "代码格式化完成"

## lint: 代码检查
lint:
	@echo "运行代码检查..."
	@golangci-lint run ./...
	@echo "代码检查完成"

## db-init: 初始化数据库
db-init:
	@echo "初始化数据库..."
	@psql -U postgres -d gcs_distill -f migrations/001_initial_schema.sql
	@echo "数据库初始化完成"

## docker-up: 启动 Docker Compose 环境
docker-up:
	@echo "启动 Docker Compose 环境..."
	@$(COMPOSE) up -d --build
	@echo "等待服务就绪..."
	@sleep 10
	@$(COMPOSE) ps
	@echo ""
	@echo "✅ 服务已启动！"
	@echo "API 服务: http://172.18.36.230:18080"
	@echo "健康检查: curl http://172.18.36.230:18080/health"

## docker-up-clean: 清理缓存并重新构建启动 Docker Compose 环境
docker-up-clean:
	@echo "清理缓存并重新构建 Docker Compose 环境..."
	@$(COMPOSE) build --no-cache
	@$(COMPOSE) up -d
	@echo "等待服务就绪..."
	@sleep 10
	@$(COMPOSE) ps
	@echo ""
	@echo "✅ 服务已启动！"
	@echo "API 服务: http://172.18.36.230:18080"
	@echo "健康检查: curl http://172.18.36.230:18080/health"

## docker-up-server: 重建并启动 gcs-server
docker-up-server:
	@echo "重建并启动 gcs-server..."
	@$(COMPOSE) up -d --build gcs-server
	@$(COMPOSE) ps gcs-server

## docker-up-server-clean: 清理缓存并重建启动 gcs-server
docker-up-server-clean:
	@echo "清理缓存并重建启动 gcs-server..."
	@$(COMPOSE) build --no-cache gcs-server
	@$(COMPOSE) up -d gcs-server
	@$(COMPOSE) ps gcs-server

## docker-up-worker: 重建并启动 gcs-worker-1
docker-up-worker:
	@echo "重建并启动 gcs-worker-1..."
	@$(COMPOSE) up -d --build gcs-worker-1
	@$(COMPOSE) ps gcs-worker-1

## docker-down: 停止 Docker Compose 环境
docker-down:
	@echo "停止 Docker Compose 环境..."
	@$(COMPOSE) down
	@echo "环境已停止"

## docker-logs: 查看 Docker Compose 日志
docker-logs:
	@$(COMPOSE) logs -f

## docker-build-all: 构建 Docker 镜像（Server + Worker）
docker-build-all:
	@echo "构建 Server 镜像..."
	@$(COMPOSE) build gcs-server
	@echo "构建 Worker 镜像..."
	@$(COMPOSE) build gcs-worker-1
	@echo "镜像构建完成"

## docker-prune: 清理 Docker 悬空镜像
docker-prune:
	@echo "清理 Docker <none> 悬空镜像..."
	@docker image prune -f
	@echo "悬空镜像清理完成"

## docker-builder-prune: 清理 Docker 构建缓存
docker-builder-prune:
	@echo "清理 Docker 构建缓存..."
	@docker builder prune -f
	@echo "构建缓存清理完成"

## docker-restart: 重启 Docker Compose 环境
docker-restart:
	@echo "重启 Docker Compose 容器..."
	@$(COMPOSE) restart
	@$(COMPOSE) ps

## test-integration: 运行集成测试
test-integration:
	@echo "运行集成测试..."
	@bash tests/integration/test_easydistill.sh
	@echo "集成测试完成"

## test-e2e: 运行端到端测试
test-e2e:
	@echo "运行端到端测试..."
	@bash tests/integration/test_e2e_workflow.sh
	@echo "端到端测试完成"

## test-all: 运行所有测试
test-all: test test-integration test-e2e
	@echo "所有测试完成"
