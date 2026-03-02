# Contest Submission Eligibility

## 功能概览
该模块用于在 Submit 创建提交前进行比赛资格校验。Submit 在 `contest_id` 非空时调用 Contest RPC，Contest 只做规则与资格判断，不创建提交记录。校验包含：比赛存在、时间窗口合法、题目属于比赛、用户已报名。该链路采用本地缓存 + Redis + MySQL 的三级缓存，保障高并发低延迟。

## 关键接口与数据结构
RPC 接口：
- `CheckSubmissionEligibility(contest_id, user_id, problem_id, submit_at)`  
返回 `ok / error_code / error_message`，错误码统一来自 `pkg/errors`。

关键表：
- `contests`：比赛元信息（时间窗口、状态）
- `contest_problems`：比赛题单
- `contest_participants`：参赛报名/审核状态

## 使用示例与配置说明
Submit 在创建提交前调用 RPC：
1) `contest_id` 为空直接跳过  
2) `contest_id` 非空 → 调用 Contest RPC  
3) `ok=false` 直接返回业务错误  

配置项（contest.rpc）建议：
- `contest.eligibilityCacheTTL`：资格缓存 TTL（默认 30m）
- `contest.eligibilityEmptyTTL`：空值缓存 TTL（默认 5m）
- `contest.eligibilityLocalCacheSize`：本地缓存容量
- `contest.eligibilityLocalCacheTTL`：本地缓存 TTL
