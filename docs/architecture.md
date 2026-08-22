# System Architecture

## Overview

Telegram Archive Bot is a single Go service that combines Telegram update polling with an authenticated HTTP API. The design keeps delivery inside Telegram while isolating business logic into archive, administration, AI, sharing, broadcast, and search services.

![Telegram Archive Bot architecture](./architecture.png)

The editable source is available in [`architecture.mmd`](./architecture.mmd).

## Runtime components

| Component | Responsibility |
|---|---|
| Telegram Bot API | Receives updates and delivers messages, media and ZIP bundles |
| Update polling and router | Dispatches commands, callbacks, text messages, and media uploads |
| Handlers | Translates Telegram events into validated application actions |
| Permission and moderation checks | Applies roles, bans, mutes, maintenance rules, and operation permissions |
| Archive service | Manages categories, subjects, files, ordering, metadata, and download counters |
| Admin and user services | Manages accounts, roles, moderation, settings, and audit records |
| HTTP API | Exposes authenticated archive, bundle, and AI endpoints for external clients |
| AI Gateway | Calls an OpenAI-compatible provider without exposing provider credentials |
| MongoDB | Stores persistent metadata, indexes, counters, users, permissions, and audit data |

## Main flows

### Archive browsing and download

A user sends a command or presses an inline button. The router delegates the event to a handler, the handler performs moderation and permission checks, and the archive service reads metadata from MongoDB. The final media is retrieved or delivered through the Telegram Bot API.

### Multi-file bundle

An authorized API client submits file IDs and a Telegram user ID. The API validates the API key and request limits, the bundle service reads the selected archive records, downloads each Telegram file with bounded context, creates a ZIP in temporary storage, and sends the resulting document to the target user.

### Advanced search

The user sends `/search` and enters a phrase with optional filters. The search service escapes the query, applies category, subject, type, and date constraints, sorts server-side, and returns a bounded page of results. Telegram and HTTP API clients use the same search contract.

### AI request

A user invokes `/ai` or `/summarize`, or an external client calls the AI API. The gateway validates the request, applies a timeout and rate limit, sends the request to the configured OpenAI-compatible provider, and returns a safe response without forwarding internal errors or provider secrets.

## Security boundaries

API routes require an API key and rate limiting. Administrator actions require explicit permissions rather than only a generic administrator flag. Share links use random expiring tokens. Search input is escaped and bounded, filter values are allowlisted, pagination is capped, and query execution is server-side. Temporary files used for ZIP bundling are size-capped, time-bounded, and removed after processing.

## Scaling path

The current single-process architecture is appropriate for a small and medium archive. Before processing large media volumes, move bundle creation and expensive search workloads behind a durable queue, store job status in MongoDB, and expose progress updates through Telegram. Broadcast delivery can similarly move to a rate-limited worker with retries. MongoDB transactions or soft-delete workflows should be introduced when content deletion becomes business-critical.
