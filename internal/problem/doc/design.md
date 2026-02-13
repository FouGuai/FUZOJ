# 题目服务设计

## 1. 目标与边界
- 题目服务负责题目与测试数据管理、版本发布与元信息读取。
- 判题服务只从题目服务获取 meta 信息；测试数据包独立从 MinIO 拉取。
- 高性能：判题侧使用拉取式轻量校验判断缓存是否过期。

## 2. 数据流与职责
- 管理端：创建/编辑题目 -> 上传数据包 -> 生成 manifest/config -> 发布新版本。
- 题目服务：写入 MySQL + MinIO，更新 Redis 最新版本索引。
- 判题服务：拉取最新 meta 校验版本/哈希，必要时从 MinIO 拉包并落本地。

## 3. 存储与索引
- MySQL：题目、版本、manifest 元信息。
- MinIO：测试数据包与文件（按 version 不可变）。
- Redis：最新发布版本索引与热点缓存。

### 3.1 MySQL 表设计（建议）
#### problem
- id (bigint, pk)
- title (varchar)
- status (tinyint) // 0=draft,1=published,2=archived
- owner_id (bigint)
- created_at, updated_at (datetime)

#### problem_version
- id (bigint, pk)
- problem_id (bigint, index)
- version (int) // monotonic
- state (tinyint) // 0=draft,1=published
- config_json (json)
- manifest_hash (varchar)
- data_pack_key (varchar)
- data_pack_hash (varchar)
- created_at (datetime)

#### problem_manifest
- id (bigint, pk)
- problem_version_id (bigint, unique)
- manifest_json (json)

#### problem_data_pack
- id (bigint, pk)
- problem_version_id (bigint, index)
- object_key (varchar)
- size_bytes (bigint)
- md5 (varchar)
- sha256 (varchar)
- created_at (datetime)

### 3.2 Redis 最新版本索引
- key: problem:latest:<problemId>
- value: {version, manifestHash, dataPackHash, updatedAt}

## 4. MinIO 数据包格式
```
problems/<problemId>/<version>/
  manifest.json
  config.json
  data/
    <testId>.in
    <testId>.ans
  checker/
    checker.bin
    checker.env
    checker.args
```

### 4.1 manifest.json 关键字段（映射判题输入）
- problemId, version
- ioConfig: mode, inputFileName, outputFileName
- checker: binaryPath, args, env, limits
- tests: testId, inputPath, answerPath, score, subtaskId, limits
- subtasks: id, score, strategy, stopOnFail
- hash: manifestHash, dataPackHash

### 4.2 config.json 关键字段（判题必要子集）
- problemId, version, title
- languageLimits: languageId, extraCompileFlags
- defaultLimits: timeMs, wallTimeMs, memoryMB, stackMB, outputKB, processes

## 5. 版本发布与一致性
- 发布生成新 version，Published 版本不可变。
- 发布前校验数据包与 manifest 哈希一致。
- 判题侧拉取后校验 dataPackHash，避免脏数据。

## 6. 判题侧缓存校验（拉取模式）
- 判题侧本地缓存保存：{problemId, version, manifestHash, dataPackHash}。
- 轻量校验接口：
  - GET /problems/{id}/latest
  - POST /problems/latest (batch)
- version 或 hash 不一致即过期，触发从 MinIO 拉取并更新本地缓存。
- 本地缓存需 LRU/TTL 回收，避免无界增长。

## 7. API 设计（最小集合）
### 7.1 管理侧
- POST /problems
- PUT /problems/{id}
- POST /problems/{id}/versions
- POST /problems/{id}/data
- POST /problems/{id}/publish
- GET /problems/{id}/versions

### 7.2 判题侧
- GET /problems/{id}/latest
- POST /problems/latest
- GET /problems/{id}/manifest?version=...
- GET /problems/{id}/config?version=...
- GET /objects/{dataPackKey} (MinIO or gateway)
