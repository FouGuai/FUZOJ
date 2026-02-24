# 题目模块

## 功能概览

题目模块负责题目元信息管理、题面管理与数据包上传发布，提供题目创建、题面更新/查询、最新版本查询、数据包上传会话、分片签名、完成上传与发布等能力。模块采用“Controller → Service → Repository”的分层结构，上传流程使用对象存储分片上传与幂等控制，适配大文件数据包场景。

## 关键接口或数据结构

- `ProblemController`：题目创建、删除、获取最新版本元信息的入口。
- `ProblemStatementController`：题面获取与更新入口（题面仅允许草稿版本编辑）。
- `ProblemUploadController`：数据包上传准备、分片签名、完成上传、终止上传与发布版本入口。
- `ProblemUploadService`：上传会话管理、版本分配、分片签名与完成上传的核心逻辑。
- `ProblemRepository`：题目元信息与最新版本缓存的访问层。
- `ProblemStatementRepository`：题面读写与多级缓存（本地 LRU + Redis + DB）。
- `ProblemUploadRepository`：上传会话、版本元数据、manifest 与 data pack 的持久化访问层。

## 使用示例或配置说明

典型流程：
1. 创建题目，获取 `problem_id`。
2. 更新题面到草稿版本（或在上传流程中先创建草稿版本）。
3. 调用上传准备接口，获取 `upload_id`、`object_key`、分片大小与过期时间。
4. 根据分片编号请求预签名 URL 并上传到对象存储。
5. 上传完成后提交分片列表与 manifest/config，完成合并并落库。
6. 发布版本，使其成为可见的最新题目版本（发布前要求题面已写入）。

该模块的对象存储参数（如 bucket、前缀、分片大小、会话 TTL）由服务初始化配置决定。
