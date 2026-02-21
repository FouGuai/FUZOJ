---
name: gozero-migration
description: Migrate existing Go services/modules to go-zero while preserving behavior, error semantics, and caching parity. Use when asked to refactor legacy Go code into go-zero structure, generate goctl scaffolding, or replace frameworks (e.g., Gin) with go-zero rest/zrpc without changing runtime semantics.
---

# Go-zero Migration

## Overview

Provide a repeatable migration workflow that keeps semantics identical, preserves existing cache behavior, and follows go-zero official patterns.

## Workflow

### 1) Intake and scope

- Identify services/modules to migrate.
- List current entrypoints, routes, RPC, storage, MQ, and cache usage.
- Confirm whether endpoints and response envelopes must remain unchanged.

### 2) Load repo rules and existing modules

- Read `/home/foushen.zhan/fuzoj/AGENTS.md` and comply with all constraints.
- Check “已实现的模块文档” for reuse and avoid duplicating components.

### 3) Generate goctl scaffolding (official structure)

- Use goctl for rest/rpc/model as appropriate.
- Only enable goctl model cache (`-c`) for tables that previously had caching.
- Keep generated code under `internal/<svc>/` and wire through `svc.ServiceContext`.

### 4) Migrate layers without semantic change

- Preserve call chain: Controller/Handler → Logic/Service → Repository.
- Port business logic first, then swap transport layer (Gin → go-zero rest) and config.
- Keep error codes, messages, and response envelope identical.

### 5) Cache parity and performance

- If the old code cached, keep caching with equivalent TTL/semantics.
- If the old code did not cache, do not introduce caching.
- Preserve cache-aside vs. blacklist set vs. local LRU behavior.

### 6) Validation

- Update tests or add new ones in `tests/` with table-driven style.
- Ensure docs and CLI client are updated when APIs change.

## Key Constraints (always enforce)

- Keep errors/logs/comments in English.
- No `panic` outside `main`.
- No `fmt.Println` in production paths; use structured logging.
- Use `pkg/errors` unified codes for business errors.
- Runtime config reads from config files unless explicitly allowed to use env vars.

## References

- See `references/migration_checklist.md` for a step-by-step parity checklist.
