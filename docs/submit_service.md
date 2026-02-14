# Submit Service

## 功能概览
Submit Service 面向前端提供判题入口，负责接收提交请求、持久化源码与元信息、上传源码到对象存储，并将判题任务投递到 Kafka。该服务以高吞吐、低延迟为目标，内置限流与幂等机制，支持前端轮询查询状态与历史源码查看，避免重复提交与缓存穿透。

## 关键接口与数据结构
核心接口包括提交与查询：
- `POST /api/v1/submissions`：创建提交并投递判题，返回 `submission_id` 与初始状态。
- `GET /api/v1/submissions/{id}`：查询单个判题状态。
- `POST /api/v1/submissions/batch_status`：批量查询状态，返回 `items` 与 `missing`。
- `GET /api/v1/submissions/{id}/source`：获取历史源码。

Kafka 消息使用 `internal/judge/model.JudgeMessage`，根据 `scene` 路由到 `judge.level0-3` 主题。

## 使用示例与配置说明
服务配置位于 `configs/submit_service.yaml`，支持配置：
- 源码大小上限、幂等 TTL、批量查询上限
- 限流窗口与阈值
- Redis/MySQL/MinIO/Kafka 连接信息

源码上传后会在数据库中持久化，并写入 Redis 缓存（TTL 30 分钟）。判题状态写入 Redis，前端通过轮询接口获取实时进度。
