GCS-Distill 环境变量配置参考
=====================================

本文档列出了 GCS-Distill 系统支持的所有环境变量。

## Control 节点（Server）环境变量

### 基础配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `CONFIG_PATH` | 配置文件路径 | `config.yaml` | 否 |
| `SERVER_HOST` | HTTP 服务监听地址 | `0.0.0.0` | 否 |
| `SERVER_PORT` | HTTP 服务端口 | `8080` | 否 |
| `SERVER_MODE` | 运行模式 (debug/release/test) | `release` | 否 |

### 数据库配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `DB_HOST` | PostgreSQL 主机地址 | `localhost` | 否 |
| `DB_PORT` | PostgreSQL 端口 | `5432` | 否 |
| `DB_USER` | 数据库用户名 | `postgres` | 否 |
| `DB_PASSWORD` | 数据库密码 | `postgres` | 否 |
| `DB_NAME` | 数据库名称 | `gcs_distill` | 否 |
| `DB_SSLMODE` | SSL 模式 | `disable` | 否 |
| `DB_MAX_CONNS` | 最大连接数 | `25` | 否 |
| `DB_MIN_CONNS` | 最小连接数 | `5` | 否 |

### Redis 配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `REDIS_HOST` | Redis 主机地址 | `localhost` | 否 |
| `REDIS_PORT` | Redis 端口 | `6379` | 否 |
| `REDIS_PASSWORD` | Redis 密码 | `` | 否 |
| `REDIS_DB` | Redis 数据库编号 | `0` | 否 |
| `REDIS_POOL_SIZE` | 连接池大小 | `10` | 否 |

### 存储配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `STORAGE_TYPE` | 存储类型 (local/nfs/ceph) | `local` | 否 |
| `STORAGE_BASE_PATH` | 工作空间基础路径 | `/mnt/shared/distill` | 否 |

### gRPC 配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `GRPC_PORT` | gRPC 服务端口 | `50051` | 否 |
| `GRPC_MAX_RECV_MSG_SIZE` | 最大接收消息大小 (MB) | `10` | 否 |
| `GRPC_MAX_SEND_MSG_SIZE` | 最大发送消息大小 (MB) | `10` | 否 |
| `GRPC_CONNECTION_TIMEOUT` | 连接超时时间 (秒) | `30` | 否 |

### 日志配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `LOG_LEVEL` | 日志级别 (debug/info/warn/error) | `info` | 否 |
| `LOG_OUTPUT` | 日志输出方式 (stdout/file) | `stdout` | 否 |
| `LOG_FILE_PATH` | 日志文件路径 | `/var/log/gcs-distill/server.log` | 否 |
| `LOG_MAX_SIZE` | 单个日志文件最大大小 (MB) | `100` | 否 |
| `LOG_MAX_AGE` | 日志保留天数 | `7` | 否 |
| `LOG_COMPRESS` | 是否压缩旧日志 | `true` | 否 |

## Worker 节点环境变量

### 基础配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `CONFIG_PATH` | 配置文件路径 | `config.yaml` | 否 |
| `NODE_NAME` | Worker 节点名称 | - | **是** |
| `NODE_ADDR` | Worker 节点地址 (host:port) | - | **是** |

### 心跳配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `HEARTBEAT_INTERVAL` | 心跳上报间隔 (秒) | `30` | 否 |
| `CLEANUP_INTERVAL` | 容器清理间隔 (秒) | `300` | 否 |

### 资源配置

| 环境变量 | 说明 | 默认值 | 必需 |
|---------|------|--------|------|
| `TOTAL_GPU` | 总 GPU 数量 | 自动检测 | 否 |
| `TOTAL_MEMORY_GB` | 总内存大小 (GB) | 自动检测 | 否 |
| `TOTAL_CPU` | 总 CPU 核心数 | 自动检测 | 否 |

## Docker Compose 示例

### 完整配置示例

```yaml
version: '3.8'

services:
  gcs-server:
    image: gcs-distill-server:latest
    environment:
      # 基础配置
      - CONFIG_PATH=/app/config.yaml
      - SERVER_HOST=0.0.0.0
      - SERVER_PORT=8080
      - SERVER_MODE=release

      # 数据库配置
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=postgres
      - DB_PASSWORD=your_secure_password
      - DB_NAME=gcs_distill

      # Redis 配置
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=your_redis_password

      # 存储配置
      - STORAGE_TYPE=nfs
      - STORAGE_BASE_PATH=/mnt/nfs/distill

      # 日志配置
      - LOG_LEVEL=info
      - LOG_OUTPUT=stdout
    ports:
      - "8080:8080"
      - "50051:50051"

  gcs-worker-1:
    image: gcs-distill-worker:latest
    environment:
      - CONFIG_PATH=/app/config.yaml
      - NODE_NAME=worker-1
      - NODE_ADDR=gcs-worker-1:50052
      - HEARTBEAT_INTERVAL=30
      - CLEANUP_INTERVAL=300
    ports:
      - "50052:50052"
```

### 最小化配置示例

```yaml
version: '3.8'

services:
  gcs-server:
    image: gcs-distill-server:latest
    environment:
      - CONFIG_PATH=/app/config.yaml
    ports:
      - "8080:8080"

  gcs-worker-1:
    image: gcs-distill-worker:latest
    environment:
      - CONFIG_PATH=/app/config.yaml
      - NODE_NAME=worker-1
      - NODE_ADDR=gcs-worker-1:50052
    ports:
      - "50052:50052"
```

## 命令行参数

### Server 命令行参数

```bash
./server [options]

Options:
  -config string
        配置文件路径 (默认 "config.yaml")
  -version
        显示版本信息
```

### Worker 命令行参数

```bash
./worker [options]

Options:
  -config string
        配置文件路径 (默认 "config.yaml")
  -name string
        Worker 节点名称 (必需)
  -addr string
        Worker 节点地址，格式: host:port (必需)
  -version
        显示版本信息
```

## 环境变量优先级

配置加载优先级（从高到低）：

1. 命令行参数
2. 环境变量
3. 配置文件
4. 默认值

示例：

```bash
# 命令行参数优先级最高
./server -config /etc/gcs/config.yaml

# 环境变量次之
export CONFIG_PATH=/etc/gcs/config.yaml
./server

# 配置文件中的值
# config.yaml: server.port: 8080

# 如果都未指定，使用默认值
```

## 生产环境最佳实践

### 1. 使用环境变量管理敏感信息

```yaml
services:
  gcs-server:
    environment:
      - DB_PASSWORD=${DB_PASSWORD}
      - REDIS_PASSWORD=${REDIS_PASSWORD}
```

然后使用 `.env` 文件：

```bash
# .env
DB_PASSWORD=your_secure_password
REDIS_PASSWORD=your_redis_password
```

### 2. 使用 Docker Secrets (Swarm)

```yaml
version: '3.8'

services:
  gcs-server:
    secrets:
      - db_password
      - redis_password
    environment:
      - DB_PASSWORD_FILE=/run/secrets/db_password
      - REDIS_PASSWORD_FILE=/run/secrets/redis_password

secrets:
  db_password:
    external: true
  redis_password:
    external: true
```

### 3. 使用配置管理工具

- Kubernetes ConfigMap 和 Secret
- Consul
- etcd
- AWS Systems Manager Parameter Store
- Azure Key Vault

## 验证配置

### 检查环境变量

```bash
# 查看容器环境变量
docker-compose exec gcs-server env | grep -E 'DB_|REDIS_|SERVER_'

# 查看配置加载情况（查看日志）
docker-compose logs gcs-server | grep -i config
```

### 测试连接

```bash
# 测试数据库连接
docker-compose exec gcs-server sh -c 'wget -qO- http://localhost:8080/health'

# 测试 Redis 连接
docker-compose exec redis redis-cli ping

# 测试 Worker 连接
curl http://localhost:8080/api/v1/nodes
```

## 故障排查

### 问题：环境变量未生效

**解决方法**：

1. 检查环境变量拼写是否正确
2. 确认 Docker Compose 文件格式正确
3. 重启容器使环境变量生效

```bash
docker-compose down
docker-compose up -d
```

### 问题：配置文件路径错误

**解决方法**：

```bash
# 检查配置文件是否正确挂载
docker-compose exec gcs-server cat /app/config.yaml

# 检查配置文件路径环境变量
docker-compose exec gcs-server env | grep CONFIG_PATH
```

## 参考资料

- [部署指南](./deployment.md)
- [配置文件说明](../config.example.yaml)
- [Docker Compose 文档](https://docs.docker.com/compose/)
