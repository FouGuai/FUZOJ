# ELK 日志体系

## 功能概览

本项目采用结构化日志 + 集中检索的企业级方案。服务日志统一输出为 JSON，并通过采集器汇总到 Elasticsearch，使用 Kibana 检索与可视化。该体系重点解决跨服务排障、错误码统计、延迟分析与访问审计等问题，并依赖 `trace_id/request_id/user_id` 做全链路关联。日志字段统一后，可按服务、环境与链路快速过滤，降低定位成本。

## 关键接口与字段

日志通过 `pkg/utils/logger` 输出，固定字段包含：`time/level/msg/caller/func/stacktrace`。配置项额外支持 `service/env/cluster`，用于索引与聚合维度。HTTP 访问日志建议包含 `method/path/status/latency/client_ip/user_id`；错误日志建议包含 `code/message/details/stack`。Trace 中间件会把 `X-Trace-Id/X-Request-Id/X-User-Id` 写入 context，保证下游服务日志可关联；当请求缺少 `X-Request-Id` 时会生成并回写到响应头。

## 使用示例与配置说明

服务配置文件中设置：
- `logger.format: json`
- `logger.outputPath: stdout`
- `logger.service/env` 指定服务名与环境名  

采集端可使用 `deploy/filebeat/filebeat.yml`，并将索引命名为 `fuzoj-logs-<env>-yyyy.MM.dd`。在 Kibana 中可按 `trace_id` 查询单条链路，或用 `msg="request error"` 聚合错误码。对于延迟分析，可使用 `msg="request completed"` 并统计 `latency` 的 P95/P99。
