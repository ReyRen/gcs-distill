# GCS-Distill 快速启动指南

本指南帮助您在 5 分钟内快速启动并运行 GCS-Distill 系统。

## 前置要求

- ✅ Docker Engine 20.10+
- ✅ Docker Compose 2.0+
- ✅ 8GB+ 可用内存
- ✅ 20GB+ 可用磁盘空间

## 快速启动步骤

### 1️⃣ 克隆仓库

```bash
git clone https://github.com/ReyRen/gcs-distill.git
cd gcs-distill
```

### 2️⃣ 启动所有服务

```bash
# 一键启动：PostgreSQL + Redis + Control 节点 + Worker 节点
docker-compose up -d

# 等待服务启动（约 30-60 秒）
docker-compose ps
```

预期输出：

```
NAME                COMMAND                  SERVICE             STATUS              PORTS
gcs-postgres        "docker-entrypoint.s…"   postgres            running (healthy)   0.0.0.0:5432->5432/tcp
gcs-redis           "docker-entrypoint.s…"   redis               running (healthy)   0.0.0.0:6379->6379/tcp
gcs-server          "./server -config /a…"   gcs-server          running             0.0.0.0:8080->8080/tcp, 0.0.0.0:50051->50051/tcp
gcs-worker-1        "sh -c ./worker -con…"   gcs-worker-1        running             0.0.0.0:50052->50052/tcp
```

### 3️⃣ 验证部署

```bash
# 检查 API 服务健康状态
curl http://172.18.36.230:18080/health

# 预期输出：
# {"status":"ok","timestamp":"2024-01-01T00:00:00Z"}

# 检查 Worker 节点注册
curl http://172.18.36.230:18080/api/v1/nodes

# 预期输出包含 worker-1 节点信息
```

### 4️⃣ 创建第一个项目

```bash
# 创建测试项目
curl -X POST http://172.18.36.230:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-first-project",
    "description": "我的第一个模型蒸馏项目",
    "teacher_model": "gpt-4",
    "student_model": "llama-7b"
  }'

# 查看项目列表
curl http://172.18.36.230:18080/api/v1/projects
```

## 🎉 完成！

现在您已经成功启动了 GCS-Distill 系统！

## 下一步

### 📖 了解更多

- [完整部署指南](./deployment.md) - 生产环境部署
- [API 文档](../README.md#api-接口) - 查看所有 API 接口
- [架构说明](../README.md#系统架构) - 理解系统设计

### 🧪 开始使用

#### 上传种子数据

```bash
# 准备种子数据文件 seeds.json
cat > seeds.json <<EOF
{"instruction": "什么是人工智能？", "input": "", "output": ""}
{"instruction": "解释机器学习的概念", "input": "", "output": ""}
EOF

# 上传种子数据（需要先获取项目ID）
PROJECT_ID=$(curl -s http://172.18.36.230:18080/api/v1/projects | jq -r '.[0].id')

curl -X POST http://172.18.36.230:18080/api/v1/projects/$PROJECT_ID/runs \
  -F "file=@seeds.json" \
  -F "teacher_model=gpt-4" \
  -F "student_model=llama-7b"
```

#### 启动蒸馏任务

```bash
# 获取 Run ID
RUN_ID=$(curl -s http://172.18.36.230:18080/api/v1/projects/$PROJECT_ID/runs | jq -r '.[0].id')

# 启动推理阶段
curl -X POST http://172.18.36.230:18080/api/v1/runs/$RUN_ID/start
```

#### 查看任务进度

```bash
# 查看 Run 状态
curl http://172.18.36.230:18080/api/v1/runs/$RUN_ID

# 查看阶段日志
curl http://172.18.36.230:18080/api/v1/runs/$RUN_ID/logs?stage=infer
```

## 常用命令

### 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f gcs-server
docker-compose logs -f gcs-worker-1

# 查看最近 100 行日志
docker-compose logs --tail=100 gcs-server
```

### 重启服务

```bash
# 重启所有服务
docker-compose restart

# 重启特定服务
docker-compose restart gcs-server
docker-compose restart gcs-worker-1
```

### 停止服务

```bash
# 停止所有服务
docker-compose down

# 停止并删除数据卷（清除所有数据）
docker-compose down -v
```

### 重新构建镜像

```bash
# 重新构建并启动
docker-compose up -d --build

# 仅构建镜像
docker-compose build
```

## 故障排查

### ❌ 服务无法启动

**问题**: 容器启动失败或立即退出

**解决方法**:

```bash
# 查看详细日志
docker-compose logs gcs-server

# 检查配置文件
cat config.yaml

# 验证数据库连接
docker-compose exec postgres psql -U postgres -c "SELECT 1;"
```

### ❌ API 返回 502/504

**问题**: API 请求超时或网关错误

**解决方法**:

```bash
# 检查服务状态
docker-compose ps

# 等待数据库就绪
docker-compose logs postgres | grep "ready to accept connections"

# 重启 API 服务
docker-compose restart gcs-server
```

### ❌ Worker 节点未注册

**问题**: `/api/v1/nodes` 返回空列表

**解决方法**:

```bash
# 检查 Worker 日志
docker-compose logs gcs-worker-1

# 检查 Redis 连接
docker-compose exec redis redis-cli ping

# 手动查看 Redis 中的节点数据
docker-compose exec redis redis-cli KEYS "node:*"
```

### ❌ 端口冲突

**问题**: `bind: address already in use`

**解决方法**:

```bash
# 检查端口占用
sudo lsof -i :8080
sudo lsof -i :5432
sudo lsof -i :6379

# 修改 docker-compose.yml 端口映射
# 将 "8080:8080" 改为 "8081:8080"
```

## 性能优化建议

### 🚀 生产环境配置

1. **增加 Worker 节点**

   编辑 `docker-compose.yml`，取消注释 `gcs-worker-2` 部分并启动：

   ```bash
   docker-compose up -d gcs-worker-2
   ```

2. **调整资源限制**

   在 `docker-compose.yml` 中添加：

   ```yaml
   services:
     gcs-server:
       deploy:
         resources:
           limits:
             cpus: '2'
             memory: 4G
           reservations:
             cpus: '1'
             memory: 2G
   ```

3. **启用持久化存储**

   将数据绑定到大盘目录，而不是落到 Docker 默认数据目录：

   ```yaml
   services:
     postgres:
       volumes:
         - /storage-md0/renyuan/gcs-distill-data/postgres:/var/lib/postgresql/data
     redis:
       volumes:
         - /storage-md0/renyuan/gcs-distill-data/redis:/data
     gcs-server:
       volumes:
         - /storage-md0/renyuan/gcs-distill-data/shared-workspace:/mnt/shared/distill
         - /storage-md0/renyuan/gcs-distill-data/logs:/var/log/gcs-distill
   ```

4. **配置反向代理**

   使用 Nginx 作为反向代理：

   ```bash
   # 创建 nginx.conf
   # 启动 Nginx 容器
   docker-compose -f docker-compose.yml -f docker-compose.nginx.yml up -d
   ```

## 获取帮助

- 📚 [完整文档](../README.md)
- 🐛 [报告问题](https://github.com/ReyRen/gcs-distill/issues)
- 💬 [讨论区](https://github.com/ReyRen/gcs-distill/discussions)

## 清理环境

如果需要完全清理环境：

```bash
# 停止并删除所有容器、网络、卷
docker-compose down -v

# 删除镜像
docker rmi gcs-distill-server:latest gcs-distill-worker:latest

# 清理构建缓存
docker builder prune -a
```

---

**祝您使用愉快！** 🎊
