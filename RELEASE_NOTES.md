# Release Notes

## v1.0.0 — Professional Go Bot

This release contains the professional Go implementation of Telegram Archive Bot.

### Included

The bot includes Telegram archive browsing and uploads, role-based administration, document-based file delivery for all media including images, secure share links, authenticated HTTP API endpoints, OpenAI-compatible AI commands, and ZIP bundling.

### Security and operations

The service uses API-key authentication and rate limiting for HTTP API routes, per-user state protection, bounded external requests, non-root Docker execution, and environment-based secrets. File conversion and FFmpeg are intentionally not included in this release.

### Verification

The release was verified with `go test ./...` and `go build ./...` using Go 1.23.
