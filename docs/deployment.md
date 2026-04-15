# GCS-Distill 部署指南

本文档介绍如何部署和运行 GCS-Distill 模型蒸馏平台。

## 目录

- [系统要求](#系统要求)
- [快速开始](#快速开始)
- [配置说明](#配置说明)
- [启动服务](#启动服务)
- [验证部署](#验证部署)
- [扩展 Worker 节点](#扩展-worker-节点)
- [故障排查](#故障排查)

## 系统要求

### 硬件要求

- **Control 节点（Server）**:
  - CPU: 2 核以上
  - 内存: 4GB 以上
  - 磁盘: 20GB 以上

- **Worker 节点**:
  - CPU: 4 核以上
  - 内存: 8GB 以上（推荐 16GB+）
  - GPU: NVIDIA GPU（可选，用于加速推理和训练）
  - 磁盘: 100GB 以上

### 软件要求

- Docker Engine 20.10+
- Docker Compose 2.0+
- （Worker 节点需要）NVIDIA Docker Runtime（如果使用 GPU）

## 快速开始

### 1. 克隆仓库

```bash
git clone https://github.com/ReyRen/gcs-distill.git
cd gcs-distill
```

### 2. 准备配置文件

```bash
# 配置文件已经准备好，位于 config.yaml
# 如需自定义，请参考 "配置说明" 章节
```

### 3. 启动所有服务

```bash
# 使用 Docker Compose 启动完整环境
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f
```

### 4. 访问 API 服务

API 服务将在 `http://172.18.36.230:18080` 上运行。

验证服务是否启动：

```bash
curl http://172.18.36.230:18080/health
```

预期响应：
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T00:00:00Z"
}
```

## 配置说明

### config.yaml 配置项

```yaml
# HTTP 服务配置
server:
  host: 0.0.0.0          # 监听地址，Docker 中使用 0.0.0.0
  port: 8080             # HTTP API 端口
  mode: release          # 运行模式: debug, release, test

# 数据库配置
database:
  host: postgres         # 数据库主机（Docker Compose 服务名）
  port: 5432
  user: postgres
  password: postgres     # 生产环境请修改密码
  dbname: gcs_distill
  sslmode: disable
  max_conns: 25
  min_conns: 5

# Redis 配置
redis:
  host: redis            # Redis 主机（Docker Compose 服务名）
  port: 6379
  password: ""           # 生产环境建议设置密码
  db: 0
  pool_size: 10

# 共享存储配置
storage:
  type: local            # 存储类型: local, nfs, ceph
  base_path: /mnt/shared/distill  # 容器内路径，映射到宿主机 /storage-md0/renyuan/gcs-distill-data/shared-workspace

# gRPC 配置
grpc:
  port: 50051            # gRPC 端口
  max_recv_msg_size: 10  # 最大接收消息大小 (MB)
  max_send_msg_size: 10  # 最大发送消息大小 (MB)
  connection_timeout: 30 # 连接超时 (秒)

# 日志配置
logging:
  level: info            # 日志级别: debug, info, warn, error
  output: stdout         # 输出方式: stdout, file
  file_path: /var/log/gcs-distill/server.log  # 容器内路径，映射到宿主机 /storage-md0/renyuan/gcs-distill-data/logs/server.log
  max_size: 100          # 单个日志文件最大大小 (MB)
  max_age: 7             # 日志保留天数
  compress: true         # 是否压缩旧日志

# 执行器配置
executor:
  workspace_root: /mnt/shared/distill  # 工作空间根目录（与 storage.base_path 一致）
  max_concurrent: 5      # 最大并发执行的流水线数量
```

**执行器配置说明**：
- `workspace_root`: 流水线执行时的工作空间根目录，应与 `storage.base_path` 保持一致
- `max_concurrent`: 同时执行的流水线数量上限。设置过高可能导致资源耗尽，设置过低会降低并发能力
  - 推荐值：5-10（根据 Worker 节点的 GPU 和内存资源调整）
  - 每个流水线执行时会占用一个 Worker 节点的资源
```

### 环境变量

Docker Compose 中使用的环境变量：

| 环境变量 | 说明 | 默认值 |
|---------|------|--------|
| `CONFIG_PATH` | 配置文件路径 | `/app/config.yaml` |
| `NODE_NAME` | Worker 节点名称 | `worker-1` |
| `NODE_ADDR` | Worker 节点地址 | `gcs-worker-1:50052` |

## 启动服务

### 启动所有服务

```bash
docker-compose up -d
```

### 启动指定服务

```bash
# 仅启动数据库和缓存
docker-compose up -d postgres redis

# 启动 Control 节点
docker-compose up -d gcs-server

# 启动 Worker 节点
docker-compose up -d gcs-worker-1
```

### 查看服务状态

```bash
# 查看所有服务状态
docker-compose ps

# 查看特定服务日志
docker-compose logs -f gcs-server
docker-compose logs -f gcs-worker-1

# 实时查看所有日志
docker-compose logs -f
```

### 停止服务

```bash
# 停止所有服务
docker-compose down

# 停止服务并删除数据卷（注意：会清除所有数据）
docker-compose down -v
```

## 验证部署

### 1. 检查服务健康状态

```bash
# 检查 API 服务
curl http://172.18.36.230:18080/health

# 检查数据库连接
docker-compose exec gcs-server sh -c 'wget -qO- http://localhost:8080/health'

# 检查 Worker 节点注册
curl http://172.18.36.230:18080/api/v1/nodes
```

### 2. 创建测试项目

```bash
# 创建项目
curl -X POST http://172.18.36.230:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-project",
    "description": "测试项目",
    "teacher_model": "gpt-4",
    "student_model": "llama-7b"
  }'

# 查看项目列表
curl http://172.18.36.230:18080/api/v1/projects
```

### 3. 检查 Worker 节点

```bash
# 查看 Worker 节点状态
curl http://172.18.36.230:18080/api/v1/nodes

# 预期输出包含 worker-1 节点信息
```

## 扩展 Worker 节点

### 方式 1: 使用 Docker Compose 扩展

编辑 `docker-compose.yml`，取消注释 `gcs-worker-2` 部分：

```yaml
  gcs-worker-2:
    build:
      context: .
      dockerfile: docker/Dockerfile.worker
    container_name: gcs-worker-2
    depends_on:
      - redis
      - gcs-server
    environment:
      - CONFIG_PATH=/app/config.yaml
      - NODE_NAME=worker-2
      - NODE_ADDR=gcs-worker-2:50053
    ports:
      - "50053:50053"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - /storage-md0/renyuan/gcs-distill-data/shared-workspace:/mnt/shared/distill
      - /var/run/docker.sock:/var/run/docker.sock
      - /storage-md0/renyuan/gcs-distill-data/logs:/var/log/gcs-distill
    networks:
      - gcs-network
    restart: unless-stopped
    privileged: true
```

启动新节点：

```bash
docker-compose up -d gcs-worker-2
```

### 方式 2: 在独立主机上运行 Worker

在其他机器上运行 Worker 节点：

```bash
# 创建 config.yaml，修改 redis 和 storage 配置指向 Control 节点

# 运行 Worker
docker run -d \
  --name gcs-worker-2 \
  --privileged \
  -p 50053:50053 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  -v /storage-md0/renyuan/gcs-distill-data/shared-workspace:/mnt/shared/distill \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e NODE_NAME=worker-2 \
  -e NODE_ADDR=<主机IP>:50053 \
  gcs-distill-worker:latest
```

## GPU 支持

如果需要在 Worker 节点上使用 GPU：

### 1. 安装 NVIDIA Docker Runtime

```bash
# 安装 NVIDIA Container Toolkit
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list

sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

### 2. 修改 Docker Compose 配置

在 `docker-compose.yml` 中为 Worker 节点添加 GPU 配置：

```yaml
  gcs-worker-1:
    # ... 其他配置 ...
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

### 3. 验证 GPU 访问

```bash
docker-compose exec gcs-worker-1 nvidia-smi
```

## 故障排查

### 问题 1: 数据库连接失败

**错误**: `connection refused` 或 `database does not exist`

**解决方法**:

```bash
# 检查数据库服务是否启动
docker-compose ps postgres

# 查看数据库日志
docker-compose logs postgres

# 手动初始化数据库
docker-compose exec postgres psql -U postgres -c "CREATE DATABASE gcs_distill;"
```

### 问题 2: Worker 节点未注册

**错误**: API 返回的节点列表为空

**解决方法**:

```bash
# 检查 Worker 日志
docker-compose logs gcs-worker-1

# 检查 Redis 连接
docker-compose exec redis redis-cli ping

# 手动测试心跳
docker-compose exec gcs-worker-1 /app/worker -config /app/config.yaml -name worker-test -addr gcs-worker-1:50052
```

### 问题 3: 容器权限问题

**错误**: `permission denied` 访问 Docker socket

**解决方法**:

```bash
# 确保 Worker 容器有 privileged 权限
# 检查 docker-compose.yml 中是否设置了:
privileged: true

# 或者修改 Docker socket 权限
sudo chmod 666 /var/run/docker.sock
```

### 问题 4: 共享存储访问问题

**错误**: 无法读写工作空间

**解决方法**:

```bash
# 检查挂载点权限
docker-compose exec gcs-server ls -la /mnt/shared/distill

# 修改权限
docker-compose exec gcs-server mkdir -p /mnt/shared/distill
docker-compose exec gcs-server chmod -R 777 /mnt/shared/distill
```

### 问题 5: 端口冲突

**错误**: `bind: address already in use`

**解决方法**:

```bash
# 检查端口占用
sudo netstat -tulpn | grep -E '8080|5432|6379|50051|50052'

# 修改 docker-compose.yml 中的端口映射
# 例如将 8080:8080 改为 8081:8080
```

## 生产环境建议

1. **修改默认密码**: 修改 PostgreSQL 和 Redis 的默认密码
2. **启用 HTTPS**: 在 API 服务前添加 Nginx/Traefik 反向代理
3. **数据备份**: 定期备份 PostgreSQL 数据库和共享存储
4. **监控告警**: 集成 Prometheus + Grafana 监控
5. **资源限制**: 为容器设置 CPU 和内存限制
6. **日志管理**: 使用 ELK/Loki 等日志收集系统
7. **高可用**: 部署多个 Control 节点（需要负载均衡）

## 下一步

- [API 文档](./api-reference.md) - 查看完整的 API 接口文档
- [开发指南](../README.md) - 了解系统架构和开发流程
- [配置参考](./configuration.md) - 详细的配置说明

## 支持

如有问题，请提交 [GitHub Issue](https://github.com/ReyRen/gcs-distill/issues)。
