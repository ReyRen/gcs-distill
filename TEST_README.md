# GCS-Distill 接口测试快速指南

## 🚀 快速开始

按照以下三个步骤完成接口测试：

### 步骤 1: 启动服务

```bash
# 方法一：使用 Make 命令（推荐）
make docker-up

# 方法二：直接使用 Docker Compose
docker-compose up -d

# 等待30秒让服务完全启动
```

### 步骤 2: 验证服务状态

```bash
# 检查所有容器是否正常运行
docker-compose ps

# 健康检查
curl http://localhost:18080/health
# 期望返回: {"status":"ok"}
```

### 步骤 3: 运行完整测试

```bash
# 方法一：使用 Make 命令（推荐）
make test-api

# 方法二：直接运行测试脚本
./test-apis.sh
```

## 📋 测试内容

自动化测试脚本会测试以下接口：

### ✅ 项目管理 API（5个接口）
- 创建项目
- 获取项目列表
- 获取项目详情
- 更新项目
- （删除项目 - 保留未测试）

### ✅ 数据集管理 API（4个接口）
- 创建数据集（JSON方式）
- 创建数据集（文件上传）
- 获取数据集列表
- 获取数据集详情

### ✅ 流水线管理 API（6个接口）
- 创建流水线
- 获取流水线列表
- 获取流水线详情
- 获取流水线阶段列表（验证6个阶段）
- 启动流水线
- 取消流水线

### ✅ 资源管理 API（2个接口）
- 获取 Worker 节点列表
- 获取节点详情

**总计：约18个测试用例**

## 📊 测试输出示例

```
==========================================
GCS-Distill API 综合测试
==========================================

Step 0: 检查服务健康状态...
----------------------------------------
✓ PASS - 服务健康检查

第一部分：项目管理 API 测试
==========================================
测试 1.1: 创建项目
----------------------------------------
✓ PASS - 创建项目成功，ID: abc123...

测试 1.2: 获取项目列表
----------------------------------------
✓ PASS - 获取项目列表成功

...

==========================================
测试总结
==========================================
总测试数: 18
通过: 18
失败: 0

🎉 所有测试通过！
```

## 🔍 快速测试（仅健康检查）

如果只想快速验证服务是否正常：

```bash
make test-api-quick
```

这会执行：
1. 健康检查
2. 项目列表接口
3. 资源节点接口

## 📖 详细文档

更详细的测试指南请参考：
- [API 测试完整指南](docs/api-testing-guide.md) - 包含手动测试步骤、场景测试、故障排查等
- [API 接口参考](docs/api-reference.md) - 所有接口的详细说明
- [前端对接指南](docs/frontend-guide.md) - 前端开发人员参考

## 🐛 常见问题

### Q1: 测试脚本提示"服务未响应"

**A:** 服务可能还在启动中，请等待30秒后重试，或检查容器状态：
```bash
docker-compose ps
docker-compose logs gcs-server
```

### Q2: 所有测试失败

**A:** 检查是否正确启动了服务：
```bash
# 停止现有服务
make docker-down

# 重新启动
make docker-up

# 等待30秒后重试
sleep 30
make test-api
```

### Q3: 部分测试失败（如数据集上传）

**A:** 可能是权限或路径问题：
```bash
# 检查共享存储目录
sudo mkdir -p /storage-md0/renyuan/gcs-distill-data/shared-workspace
sudo chmod 777 /storage-md0/renyuan/gcs-distill-data/shared-workspace
```

### Q4: Worker 节点不在线

**A:** 检查 Worker 容器和 Redis 连接：
```bash
docker-compose logs gcs-worker-1
docker-compose exec redis redis-cli
> KEYS worker:*
```

## 🛠️ 手动测试示例

### 创建项目

```bash
curl -X POST http://localhost:18080/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的测试项目",
    "description": "测试描述",
    "teacher_model_config": {
      "provider_type": "local",
      "model_name": "Qwen/Qwen2.5-7B-Instruct"
    },
    "student_model_config": {
      "provider_type": "local",
      "model_name": "Qwen/Qwen2.5-0.5B-Instruct"
    }
  }'
```

### 上传数据集

```bash
# 创建测试文件
cat > test.jsonl << EOF
{"instruction": "问题1", "input": "", "output": "答案1"}
{"instruction": "问题2", "input": "", "output": "答案2"}
EOF

# 上传（替换 {项目ID} 为实际的项目ID）
curl -X POST http://localhost:18080/api/v1/projects/{项目ID}/datasets \
  -F "file=@test.jsonl" \
  -F "name=测试数据集"
```

### 查看流水线状态

```bash
# 获取流水线详情（替换 {流水线ID} 为实际的流水线ID）
curl http://localhost:18080/api/v1/pipelines/{流水线ID} | jq

# 获取6个阶段详情
curl http://localhost:18080/api/v1/pipelines/{流水线ID}/stages | jq
```

## 📝 测试数据说明

测试脚本会自动创建以下测试数据：

1. **测试项目**
   - 名称：测试蒸馏项目
   - 教师模型：Qwen/Qwen2.5-7B-Instruct
   - 学生模型：Qwen/Qwen2.5-0.5B-Instruct

2. **测试数据集**
   - 通过 JSON 创建：100条记录
   - 通过文件上传：3条测试问答对

3. **测试流水线**
   - 包含完整的训练配置
   - 启用 LoRA 微调
   - 请求1个GPU

测试完成后，这些数据会保留在数据库中，可以用于后续的手动验证。

## 🧹 清理测试数据

如果需要清理所有测试数据：

```bash
# 完全清理（删除所有数据和容器）
docker-compose down -v

# 重新启动
docker-compose up -d
```

## 📞 获取帮助

- 查看所有 Make 命令：`make help`
- 查看服务日志：`docker-compose logs -f`
- 查看 API 文档：访问 `http://localhost:18080/swagger/index.html`

## 🎯 下一步

测试通过后，您可以：

1. **与前端团队对接**
   - 分享 `docs/frontend-guide.md`
   - 组织接口对接会议

2. **进行压力测试**
   - 使用 `ab` 或 `wrk` 工具
   - 参考 `docs/api-testing-guide.md`

3. **部署到生产环境**
   - 参考 `docs/deployment.md`
   - 配置正式的域名和 HTTPS

---

**祝测试顺利！** 🎉

如有问题，请查看 `docs/api-testing-guide.md` 获取更详细的故障排查指南。
