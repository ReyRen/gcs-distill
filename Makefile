.PHONY: all build clean test proto server worker docker-build help

# 变量定义
BINARY_SERVER=bin/gcs-distill-server
BINARY_WORKER=bin/gcs-distill-worker
VERSION=v0.1.0
DOCKER_IMAGE=gcs-distill/easydistill
DOCKER_TAG=latest

# 默认目标
all: build

## help: 显示帮助信息
help:
	@echo "GCS-Distill Makefile 命令:"
	@echo ""
	@echo "  make build          - 编译服务端和 Worker"
	@echo "  make server         - 仅编译服务端"
	@echo "  make worker         - 仅编译 Worker"
	@echo "  make proto          - 生成 gRPC 代码"
	@echo "  make test           - 运行测试"
	@echo "  make clean          - 清理编译产物"
	@echo "  make docker-build   - 构建 EasyDistill Docker 镜像"
	@echo "  make run-server     - 运行服务端"
	@echo "  make run-worker     - 运行 Worker"
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
	@protoc --go_out=. --go-grpc_out=. proto/worker.proto
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
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f docker/easydistill/Dockerfile docker/easydistill/
	@echo "镜像构建完成: $(DOCKER_IMAGE):$(DOCKER_TAG)"

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
