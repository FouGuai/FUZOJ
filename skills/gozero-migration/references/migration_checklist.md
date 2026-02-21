# Migration Checklist (Semantics and Cache Parity)

## Pre-migration audit

- Identify entrypoints, routes, and RPC methods.
- Record response envelope fields and error codes.
- Record cache locations, keys, TTLs, and cache-aside logic.
- Confirm configuration source (must remain config-file driven).

## Go-zero scaffolding

- Use `goctl api`/`goctl rpc` for handlers/logic/svc.
- Use `goctl model` for tables.
- Enable goctl model cache only if the original table access was cached.

## Handler/logic parity

- Keep request/response structs and JSON fields unchanged.
- Preserve validation rules and error code mapping.
- Keep logging fields consistent (trace_id, user_id, etc.).

## Repository/model parity

- Do not add new caches or change TTLs.
- Preserve blacklist/ban set semantics.
- Keep DB queries and transaction boundaries equivalent.

## Verification

- Update or add tests in `tests/` (table-driven).
- Ensure CLI client and docs are updated if endpoints changed.
- Re-check AGENTS.md constraints after changes.
