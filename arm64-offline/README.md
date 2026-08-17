# gcs-distill ARM64 离线部署包

本目录包含静态 ARM64 二进制，现场服务器不需要安装 Go。

## 使用方法

1. 修改 `config.toml` 中的服务、MySQL、共享存储和 GCS 地址。
2. 以 root 用户执行 `make verify`。
3. 执行 `make deploy`，默认安装到 `/opt/gcs/gcs-distill`。
4. 使用 `make status` 或 `make logs` 检查服务。

配置中的 MySQL 数据库必须已经存在，账号须具备连接及建表权限。服务启动时会通过 `CREATE TABLE IF NOT EXISTS` 自动初始化 `distill_*` 表，不需要单独拷贝迁移文件。
