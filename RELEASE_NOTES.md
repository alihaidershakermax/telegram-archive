# Release Notes

## v1.0.0 — Professional Go Bot

This release contains the professional Go implementation of Telegram Archive Bot.

### Included

The bot includes Telegram archive browsing and uploads, role-based administration, document-based file delivery for all media including images, secure share links, authenticated HTTP API endpoints, OpenAI-compatible AI commands, and ZIP bundling.

### Security and operations

The service uses API-key authentication and rate limiting for HTTP API routes, per-user state protection, bounded external requests, non-root Docker execution, and environment-based secrets. File conversion and FFmpeg are intentionally not included in this release.

### Verification

The release was verified with `go test ./...` and `go build ./...` using Go 1.23.

## Unreleased — Bot Factory and API v2

This release adds an operator-controlled Bot Factory for registering BotFather-created tokens, encrypted AES-256-GCM storage, isolated polling workers, live health metrics, a load-aware outbound router, Telegram commands `/newbot` and `/mybots`, and the authenticated `/api/v2` lifecycle endpoints. It preserves API v1 for archive, bundle, and AI clients. The factory is disabled until `FACTORY_ENCRYPTION_KEY` is configured.

The manager is process-local and suitable for one continuously running Koyeb instance. Multiple replicas require a durable queue, distributed token leases, and a planned secret-rotation migration.
