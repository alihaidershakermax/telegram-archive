# Release Notes

## v1.0.0 — Professional Go Bot

This release contains the professional Go implementation of Telegram Archive Bot.

### Included

The bot includes Telegram archive browsing and uploads, role-based administration, image-aware file delivery, secure share links, authenticated HTTP API endpoints, OpenAI-compatible AI commands, ZIP bundling, and advanced archive search.

Advanced search supports text queries, file type, category, subject, date range, bounded pagination, and sorting by newest, oldest, name, or downloads. The Telegram command is `/search`; the HTTP endpoint is `/api/v1/search`.

### Security and operations

The service uses API-key authentication and rate limiting for HTTP API routes, per-user state protection, bounded external requests, non-root Docker execution, and environment-based secrets. File conversion and FFmpeg are intentionally not included in this release.

### Verification

The release was verified with `go test ./...` and `go build ./...` using Go 1.23.
