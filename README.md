# fuzoj 调试启动说明

本文档用于本地调试环境启动，包含中间件与服务的启动/停止步骤。

## 前置依赖

- Docker / Docker Compose
- Go（与项目 `go.mod` 版本兼容）
- Python 3（用于调试脚本）

## 中间件列表

调试环境会启动以下中间件：

- MySQL
- Redis
- MinIO
- Kafka
- Etcd
- Elasticsearch
- Kibana
- Filebeat

## 一键启动（推荐）

在仓库根目录执行：

```bash
python3 scripts/debug_start.py
```

该命令会执行以下流程：

1. 启动中间件（Docker Compose）
2. 初始化 MySQL 与导入 schema
3. 创建 MinIO buckets
4. 初始化 Kafka topics
5. 构建并启动服务（默认包含 user-service）

说明：服务启动时读取 `services/*/etc/*.yaml` 的本地配置文件，运行时地址与日志配置通过配置中心（etcd）加载，建议先执行配置中心初始化。

## 配置中心初始化

使用本地 `services/*/etc/*.yaml` 作为依据，将配置写入 Etcd：

```bash
go run ./scripts/devtools/etcdinit
```

只初始化部分服务：

```bash
go run ./scripts/devtools/etcdinit --only gateway,submit,judge
```

仅查看将写入的内容：

```bash
go run ./scripts/devtools/etcdinit --dry-run
```

## 只启动中间件

```bash
python3 scripts/debug_start.py --deps-only
```

## 只启动服务

```bash
python3 scripts/debug_start.py --services-only
```

## 仅启动指定服务

```bash
python3 scripts/debug_start.py --only gateway
```

多个服务用逗号分隔，例如：

```bash
python3 scripts/debug_start.py --only gateway,judge-service
```

## 停止服务与中间件

```bash
python3 scripts/debug_stop.py
```

仅停止服务：

```bash
python3 scripts/debug_stop.py --services-only
```

仅停止中间件：

```bash
python3 scripts/debug_stop.py --deps-only
```

## 配置与端口

- 服务端口以 `services/*/etc/*.yaml` 中的配置为准
- 网关地址会同步到 `configs/cli.yaml` 的 `baseURL`

## 常见问题

1. **PID 文件残留**
   - 若进程已退出但 pid 文件仍在，请执行 `python3 scripts/debug_stop.py` 清理。

2. **端口占用**
   - 运行时端口为随机分配，但若端口冲突，请重新执行启动命令。

3. **Docker 未启动**
   - 请先启动 Docker 后再执行脚本。
