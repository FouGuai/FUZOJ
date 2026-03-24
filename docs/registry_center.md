# 注册中心与配置中心（Etcd）格式说明

## 功能概览
本文件定义服务在注册中心的注册格式与运行时配置、日志配置的存储规范。服务启动时从 Etcd 读取运行时与日志配置，再注册自身地址到 Etcd。该规范用于保证多实例可用、配置中心统一与网关后续路由扩展的一致性。

## 配置中心地址来源
服务启动时通过本地 `etc/*.yaml` 中的 `bootstrap.etcd.hosts` 获取配置中心地址。该地址不从环境变量读取，仅由启动配置文件提供。

## 关键结构与 Key 约定
1) **全量配置（方案A）**  
Key：`{service}.config`  
Value：JSON，内容与服务 `etc/*.yaml` 对应（不包含 `bootstrap`）。  
服务启动时优先从该 Key 读取完整配置，所有中间件地址以此为准。

1) **运行时配置（REST）**  
Key：`{service}.rest.runtime`  
Value：JSON，对应 `rest.RestConf` 的最小启动字段：  
`name`、`host`、`port`、`timeout`、`middlewares`，以及可选 `registerKey`。  
`timeout` 为毫秒整数（int64）。  
`registerKey` 默认是 `{service}.rest`。  
当 `port=0` 时，服务启动会自动分配随机端口并回写到该 runtime key。

2) **运行时配置（RPC）**  
Key：`{service}.rpc.runtime`  
Value：JSON，对应 `zrpc.RpcServerConf` 的最小启动字段：  
`name`、`listenOn` 与 `etcd`（含 `hosts`、`key`）。  
当 `listenOn` 端口为 `0` 时，服务启动会自动分配随机端口并回写到该 runtime key。

3) **日志配置**  
Key：`{service}.log`  
Value：JSON。  
Gateway 使用 `pkg/utils/logger.Config`；其余服务使用 `logx.LogConf`。

4) **服务注册（REST）**  
Key：`{service}.rest`（或 `registerKey` 指定）  
Value：`host:port`  
多实例通过 Etcd 租约自动生成子 Key（如 `key/{leaseId}`），无需手动维护实例 ID。

## 服务配置清单与示例
以下示例均为 `{service}.config` 的 JSON 内容，不包含 `bootstrap`。

### Gateway Service
必需字段：
- `name` `host` `port` `middlewares`（RestConf）
- `auth` `redis` `kafka` `banEvent` `cache` `rateLimit` `proxy` `cors` `upstreams`
示例：
```json
{
  "name": "gateway",
  "host": "0.0.0.0",
  "port": 8080,
  "timeout": "0s",
  "middlewares": {"recover": true},
  "logger": {"level":"info","format":"json","outputPath":"stdout","errorPath":"stderr","service":"gateway","env":"dev","cluster":"default"},
  "auth": {"jwtSecret":"tyylovezsrtyylovezsr","jwtIssuer":"fuzoj"},
  "redis": {"host":"127.0.0.1:6379","type":"node"},
  "kafka": {"brokers":["127.0.0.1:9092"],"conns":1,"consumers":2,"processors":2},
  "banEvent": {"enabled":true,"topic":"user.events","consumerGroup":"fuzoj-gateway-bans"},
  "cache": {"banLocalTTL":"30m","banLocalSize":100000,"tokenBlacklistCacheTTL":"2m"},
  "rateLimit": {"window":"1m","userMax":1200000,"ipMax":1200000,"routeMax":1200000,"globalRefillPerSec":20000,"globalCapacity":100000},
  "proxy": {"maxIdleConns":2048,"maxIdleConnsPerHost":256,"idleConnTimeout":"90s","responseHeaderTimeout":"5s","tlsHandshakeTimeout":"5s","dialTimeout":"3s"},
  "cors": {"enabled":true,"allowedOrigins":["*"],"allowedMethods":["GET","POST","PUT","DELETE","PATCH","OPTIONS"],"allowedHeaders":["Authorization","Content-Type","Idempotency-Key","X-Trace-Id"],"exposedHeaders":["X-Trace-Id","X-Request-Id"],"allowCredentials":false,"maxAge":"12h"},
  "upstreams": [
    {"name":"user","registryKey":"user.rest","http":{"target":"127.0.0.1:8081","timeout":0},
      "mappings":[{"name":"user.register","method":"POST","path":"/api/v1/user/register","auth":{"mode":"public"}}]},
    {"name":"problem","registryKey":"problem.rest","http":{"target":"127.0.0.1:8083","timeout":0},
      "mappings":[{"name":"problem.public","method":"GET","path":"/api/v1/problems/*any","auth":{"mode":"public"}}]}
  ]
}
```

### User Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `auth`
示例：
```json
{
  "name":"user",
  "host":"0.0.0.0",
  "port":8888,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "auth":{"jwtSecret":"tyylovezsrtyylovezsr","jwtIssuer":"fuzoj","accessTokenTTL":"15m","refreshTokenTTL":"168h","loginFailTTL":"15m","loginFailLimit":5,
    "root":{"enabled":true,"username":"root","password":"tyy1314520","email":"root@local"}}
}
```

### Submit Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `kafka` `minio` `topics` `submit` `contestRpc`
示例：
```json
{
  "name":"submit",
  "host":"0.0.0.0",
  "port":8086,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "kafka":{"brokers":["127.0.0.1:9092"],"clientID":"submit-service","minBytes":10240,"maxBytes":10485760},
  "minio":{"endpoint":"127.0.0.1:9000","accessKey":"minioadmin","secretKey":"minioadmin","useSSL":false,"bucket":"fuzoj"},
  "topics":{"level0":"judge.level0","level1":"judge.level1","level2":"judge.level2","level3":"judge.level3"},
  "contestRpc":{"etcd":{"hosts":["127.0.0.1:2379"],"key":"contest.rpc"}},
  "submit":{"sourceBucket":"fuzoj","sourceKeyPrefix":"submissions","maxCodeBytes":262144,"idempotencyTTL":"10m","batchLimit":200,
    "statusTTL":"24h","statusEmptyTTL":"5m","statusFinalTopic":"judge.status.final",
    "statusFinalConsumer":{"consumerGroup":"submit-service-status-final","prefetchCount":1,"concurrency":2,"maxRetries":3,"retryDelay":"1s","deadLetterTopic":"judge.status.final.dead","messageTTL":"10m"},
    "submissionCacheTTL":"30m","submissionEmptyTTL":"5m",
    "rateLimit":{"userMax":30,"ipMax":60,"window":"1m"},
    "timeouts":{"db":"3s","cache":"1s","mq":"3s","storage":"5s","status":"2s","contestRPC":"800ms"}}
}
```

### Problem Service
必需字段：
- `name` `host` `port`（RestConf）
- `rpc` `mysql` `cache` `redis` `kafka` `minio` `upload` `cleanup` `statement`
示例：
```json
{
  "name":"problem",
  "host":"0.0.0.0",
  "port":8888,
  "rpc":{"listenOn":"0.0.0.0:9093","etcd":{"hosts":["127.0.0.1:2379"],"key":"problem.rpc"}},
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "kafka":{"brokers":["127.0.0.1:9092"],"clientID":"problem-service","minBytes":10240,"maxBytes":10485760},
  "minio":{"endpoint":"127.0.0.1:9000","accessKey":"minioadmin","secretKey":"minioadmin","useSSL":false,"bucket":"problem-data","presignTTL":"15m"},
  "upload":{"keyPrefix":"problems","partSizeBytes":16777216,"sessionTTL":"2h","presignTTL":"15m"},
  "cleanup":{"topic":"problem.cleanup","consumerGroup":"problem-cleanup","prefetchCount":10,"concurrency":2,"maxRetries":3,"retryDelay":"1s","deadLetterTopic":"","messageTTL":"0s",
    "batchSize":1000,"listTimeout":"30s","deleteTimeout":"2m","maxUploads":1000},
  "statement":{"maxBytes":131072,"redisTTL":"30m","emptyTTL":"5m","localCacheSize":1024,"localCacheTTL":"5m","timeout":"2s"}
}
```

### Judge Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `kafka` `minio` `cacheConfig` `worker` `source` `problem` `status` `judge` `sandbox` `language`
注意：`minio.bucket` 必须与 Problem Service 上传数据包使用的 bucket 保持一致，否则 Judge 无法通过 data_pack_key 读取数据包。
示例：
```json
{
  "name":"judge",
  "host":"0.0.0.0",
  "port":8888,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "statusCacheTTL":"30m","statusCacheEmptyTTL":"5m",
  "kafka":{"brokers":["127.0.0.1:9092"],"clientID":"judge-service","topics":["judge.task.high","judge.task.normal"],
    "topicWeights":{"judge.task.high":8,"judge.task.normal":4},
    "consumerGroup":"judge-service","prefetchCount":10,"concurrency":4,"maxRetries":3,"retryDelay":"1s",
    "retryTopic":"judge.retry","retryMaxInFlight":1,"poolRetryMax":5,"poolRetryBaseDelay":"1s","poolRetryMaxDelay":"30s","deadLetter":"judge.dead","messageTTL":"10m"},
  "minio":{"endpoint":"127.0.0.1:9000","accessKey":"minioadmin","secretKey":"minioadmin","useSSL":false,"bucket":"judge-data","presignTTL":"15m"},
  "cacheConfig":{"rootDir":"/data/judge/cache","ttl":"30m","lockWait":"5s","maxEntries":256,"maxBytes":10737418240},
  "worker":{"poolSize":4,"timeout":"30s"},
  "source":{"bucket":"judge-sources","timeout":"10s"},
  "problemRpc":{"etcd":{"hosts":["127.0.0.1:2379"],"key":"problem.rpc"},"timeout":2000,"callTimeout":"2s","metaTTL":"30s"},
  "status":{"ttl":"30m","timeout":"2s","finalTopic":"judge.status.final","finalBatchSize":100,"finalBatchInterval":"1s","finalBatchTimeout":"3s"},
  "judge":{"workRoot":"/data/judge/work"},
  "sandbox":{"cgroupRoot":"/sys/fs/cgroup","seccompDir":"/home/foushen.zhan/fuzoj/configs/seccomp","helperPath":"/usr/local/bin/judge-helper","stdoutStderrMaxBytes":1048576,
    "enableSeccomp":true,"enableCgroup":true,"enableNamespaces":true},
  "language":{"languages":[],"profiles":[]}
}
```

### Status Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `minio` `status`
示例：
```json
{
  "name":"status",
  "host":"0.0.0.0",
  "port":8089,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "minio":{"endpoint":"127.0.0.1:9000","accessKey":"minioadmin","secretKey":"minioadmin","useSSL":false,"bucket":"fuzoj","presignTTL":"15m"},
  "status":{"cacheTTL":"30m","cacheEmptyTTL":"5m","logBucket":"fuzoj","logKeyPrefix":"submission-logs","logMaxBytes":65536,"logCacheTTL":"10m",
    "timeouts":{"db":"3s","cache":"1s","storage":"5s","status":"2s"}}
}
```

### Status SSE Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `statusSSE` `timeouts`
示例：
```json
{
  "name":"status-sse",
  "host":"0.0.0.0",
  "port":8093,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "statusSSE":{"ownerCacheTTL":"5m","ownerCacheEmptyTTL":"1m","debounce":"200ms","heartbeat":"15s"},
  "timeouts":{"db":"2s","cache":"1s","sse":"0s"}
}
```

### Contest Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `kafka` `contest` `contestDispatch` `judgeFinal` `rankUpdate` `leaderboard` `timeouts`
示例：
```json
{
  "name":"contest",
  "host":"0.0.0.0",
  "port":8087,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "kafka":{"brokers":["127.0.0.1:9092"],"clientID":"contest-service","minBytes":10240,"maxBytes":10485760,
    "topics":["judge.status.final","contest.leaderboard.rebuild","contest.hack.finished","contest.rank.updates","contest.judge.final.dead"]},
  "contest":{"idempotencyTTL":"10m","resultPersistAfter":"1h","maxParticipantsPerTeam":3,"defaultPageSize":50,"maxPageSize":200,
    "eligibilityCacheTTL":"30m","eligibilityEmptyTTL":"5m","eligibilityLocalCacheSize":2048,"eligibilityLocalCacheTTL":"10s"},
  "contestDispatch":{"topic":"contest.submit.validate","consumerGroup":"contest-dispatch","prefetchCount":1,"concurrency":4,"maxRetries":3,
    "retryDelay":"1s","deadLetterTopic":"contest.submit.validate.dead","messageTTL":"10m","idempotencyTTL":"30m","statusTTL":"24h"},
  "judgeFinal":{"topic":"judge.status.final","consumerGroup":"contest-judge-final","prefetchCount":1,"concurrency":4,"maxRetries":3,
    "retryDelay":"1s","deadLetterTopic":"contest.judge.final.dead","messageTTL":"10m","idempotencyTTL":"30m"},
  "rankUpdate":{"topic":"contest.rank.updates"},
  "leaderboard":{"hotCacheTTL":"3s","pageCacheTTL":"5s","emptyTTL":"5m","frozenCacheTTL":"10m","snapshotInterval":"5m"},
  "timeouts":{"db":"3s","cache":"1s","mq":"3s"}
}
```

### Contest RPC Service
必需字段：
- `name` `listenOn`（RpcServerConf）
- `mysql` `cache` `kafka` `contest` `leaderboard` `timeouts`
示例：
```json
{
  "name":"contest.rpc",
  "listenOn":"0.0.0.0:9094",
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "kafka":{"brokers":["127.0.0.1:9092"],"clientID":"contest-rpc-service","minBytes":10240,"maxBytes":10485760,
    "topics":["judge.status.final","contest.leaderboard.rebuild","contest.hack.finished"]},
  "contest":{"idempotencyTTL":"10m","resultPersistAfter":"1h","maxParticipantsPerTeam":3,"defaultPageSize":50,"maxPageSize":200},
  "leaderboard":{"hotCacheTTL":"3s","pageCacheTTL":"5s","emptyTTL":"5m","frozenCacheTTL":"10m","snapshotInterval":"5m"},
  "timeouts":{"db":"3s","cache":"1s","mq":"3s"}
}
```

## 使用示例
REST 运行时（`user.rest.runtime`）：
```json
{"name":"user","host":"0.0.0.0","port":8081,"timeout":"0s","middlewares":{"recover":true},"registerKey":"user.rest"}
```
RPC 运行时（`problem.rpc.runtime`）：
```json
{"listenOn":"0.0.0.0:9093","etcd":{"hosts":["127.0.0.1:2379"],"key":"problem.rpc"}}
```
日志（`user.log`）：
```json
{"serviceName":"user","mode":"console","encoding":"json","level":"info"}
```

## 更新约定
如新增或变更 Etcd 中的运行时字段、日志字段、Key 命名规则或 `{service}.config` 字段，必须同步更新本文件，保持与代码实现一致。

### Rank Service
必需字段：
- `name` `host` `port`（RestConf）
- `mysql` `cache` `redis` `kafka` `rank` `timeouts`
示例：
```json
{
  "name":"rank",
  "host":"0.0.0.0",
  "port":8091,
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "kafka":{"brokers":["127.0.0.1:9092"],"clientID":"rank-service","minBytes":10240,"maxBytes":10485760},
  "rank":{"updateTopic":"contest.rank.updates","consumerGroup":"rank-service","prefetchCount":1,"concurrency":8,
    "batchSize":200,"batchInterval":"100ms","hotCacheTTL":"3s","pageCacheTTL":"5s","emptyTTL":"5m","wsDebounce":"100ms",
    "snapshotInterval":"5m","snapshotPageSize":500,"snapshotBatch":500,"recoverOnStart":true},
  "timeouts":{"cache":"1s","db":"3s","mq":"3s"}
}
```

### Rank WS Service
必需字段：
- `name` `host` `port`（RestConf）
- `cache` `redis` `rank` `timeouts`
示例：
```json
{
  "name":"rank-ws",
  "host":"0.0.0.0",
  "port":8092,
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "rank":{"pageCacheTTL":"5s","emptyTTL":"5m","wsDebounce":"100ms"},
  "timeouts":{"cache":"1s","ws":"3s"}
}
```

### Rank RPC Service
必需字段：
- `name` `listenOn`（RpcServerConf）
- `mysql` `cache` `redis` `rank` `timeouts`
示例：
```json
{
  "name":"rank-rpc",
  "listenOn":"0.0.0.0:9096",
  "mysql":{"dataSource":"user:password@tcp(127.0.0.1:3306)/fuzoj?charset=utf8mb4&parseTime=true&loc=Local"},
  "cache":[{"host":"127.0.0.1:6379","type":"node"}],
  "redis":{"host":"127.0.0.1:6379","type":"node"},
  "rank":{"hotCacheTTL":"3s","pageCacheTTL":"5s","emptyTTL":"5m"},
  "timeouts":{"cache":"1s"}
}
```
