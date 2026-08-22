# Release Notes

## v1.0.0 — Professional Go Bot

This release contains the professional Go implementation of Telegram Archive Bot.

### Included

The bot includes Telegram archive browsing and uploads, role-based administration, document-based file delivery for all media including images, secure share links, authenticated HTTP API endpoints, OpenAI-compatible AI commands, and ZIP bundling.

### Security and operations

The service uses API-key authentication and rate limiting for HTTP API routes, per-user state protection, bounded external requests, non-root Docker execution, and environment-based secrets. File conversion and FFmpeg are intentionally not included in this release.

### Verification

The release was verified with `go test ./...` and `go build ./...` using Go 1.23.

## v2.4.0 — Lightweight Group Support and Unified Group API

This release adds the first production slice of lightweight Telegram group support. Each managed bot now stores group configuration using the isolated `bot_id` and `chat_id` namespace, and `/group` can initialize and display a group's configuration from Telegram.

API v2 adds authenticated `GET /api/v2/groups/{chat_id}` and `PATCH /api/v2/groups/{chat_id}` routes. Requests must provide `X-Telegram-Bot-ID`; scoped API keys are restricted to their registered bot and inherit `archive:read` or `archive:write` permissions. Group identifiers are validated as negative Telegram chat IDs, and configuration updates never cross bot namespaces.

The implementation intentionally leaves rate limiting, anti-flood moderation, full Telegram admin checks, and TTL settings cache for the next group-support increment.

### Verification

The complete Go suite passed with `go test ./...`, formatting and whitespace checks passed, and the secondary repository received the synchronized source and documentation changes.

## Unreleased — Bot Factory and API v2

This release adds an operator-controlled Bot Factory for registering BotFather-created tokens, encrypted AES-256-GCM storage, isolated polling workers, live health metrics, a load-aware outbound router, Telegram commands `/newbot` and `/mybots`, and the authenticated `/api/v2` lifecycle endpoints. It preserves API v1 for archive, bundle, and AI clients. The factory is disabled until `FACTORY_ENCRYPTION_KEY` is configured.

The manager is process-local and suitable for one continuously running Koyeb instance. Multiple replicas require a durable queue, distributed token leases, and a planned secret-rotation migration.
