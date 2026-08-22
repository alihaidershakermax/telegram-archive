# System Architecture

## Overview

Telegram Archive Bot is a Go service that combines a primary Telegram controller, an authenticated HTTP API, and an optional Bot Factory. The factory lets authorized operators register tokens created through BotFather, encrypts them before persistence, and restores isolated polling workers after a process restart. Archive, administration, AI, sharing, broadcast, and managed-bot lifecycle logic remain separated behind explicit services.

![Telegram Archive Bot architecture](./architecture.png)

The editable source is available in [`architecture.mmd`](./architecture.mmd).

## Runtime components

| Component | Responsibility |
|---|---|
| Telegram Bot API | Delivers updates, media, and documents for the controller and managed bots |
| Primary controller bot | Provides the Bot Factory commands and administration surface |
| Bot Factory Manager | Registers, encrypts, restores, pauses, resumes, and deletes managed bot workers |
| Managed bot workers | Run isolated long polling streams; one Telegram token maps to one worker |
| Routing policy | Selects a healthy bot for factory-managed outbound work using recency, latency, active work, and errors |
| Update router | Dispatches commands, callbacks, text messages, and media uploads |
| Handlers | Translates Telegram events into validated application actions |
| Permission and moderation checks | Applies roles, bans, mutes, maintenance rules, and operation permissions |
| Archive service | Manages categories, subjects, files, ordering, metadata, and download counters |
| Admin and user services | Manages accounts, roles, moderation, settings, and audit records |
| HTTP API v1 | Provides archive, bundle, and AI endpoints |
| HTTP API v2 | Provides managed-bot registration, lifecycle, health, and routing endpoints |
| AI Gateway | Calls an OpenAI-compatible provider without exposing provider credentials |
| MongoDB | Stores archive metadata, encrypted bot registrations, indexes, counters, users, permissions, and audit records |

## Bot Factory lifecycle

An authorized operator invokes `/newbot` on the primary controller and sends a BotFather token in a separate message. The factory validates the token with Telegram `getMe`, calculates a non-reversible duplicate-check hash, encrypts the token using AES-256-GCM, and stores only encrypted token material plus safe metadata. The plaintext token is not returned through the API and is not written to logs.

At startup, the manager restores records with `active` status, decrypts each token in memory, validates it again with Telegram, and starts an isolated long-polling worker. `/mybots` exposes safe status and counters. API v2 can pause, resume, or permanently delete a registration. `FACTORY_MAX_BOTS_PER_OWNER` limits registrations, while the configured owner is the only API registration owner in the current platform-operator model.

## Intelligent routing

Each worker records update throughput, active work, observed processing latency, health recency, consecutive failures, and accumulated errors. The routing policy scores active healthy workers and chooses the strongest candidate for factory-managed outbound jobs. Telegram updates cannot be moved between bot tokens or merged into one session; therefore the router does not pretend to rebalance an existing Telegram stream. Horizontal scaling should later add a durable queue and a distributed lease to prevent two service instances from polling the same token.

## Main flows

### Archive browsing and document download

A user sends a command or presses an inline button. The router delegates the event to a handler, the handler performs moderation and permission checks, and the archive service reads metadata from MongoDB. Archived media, including image-origin files, is sent as a Telegram document when the user downloads it, preserving the original file-oriented behavior.

### Multi-file bundle

An authorized API client submits file IDs and a Telegram user ID. The API validates the API key and request limits, the bundle service reads the selected archive records, downloads each Telegram file with bounded context, creates a ZIP in temporary storage, and sends the resulting document to the target user.

### API v2 bot registration

An authenticated operator submits a token to `POST /api/v2/bots`. The server validates the bounded JSON body, associates the record with the configured platform owner, calls Telegram, encrypts the token, persists metadata, and starts the worker. List, detail, pause, resume, delete, health, and best-router endpoints never expose ciphertext, nonce, token hash, or plaintext.

### AI request

A user invokes `/ai` or `/summarize`, or an external client calls the AI API. The gateway validates the request, applies a timeout and rate limit, sends the request to the configured OpenAI-compatible provider, and returns a safe response without forwarding internal errors or provider secrets.

## Security boundaries

API routes require an API key and rate limiting. Administrator actions require explicit permissions, and Bot Factory operations are limited to `manage_bots` operators. The encryption key is mandatory for factory persistence and must be stored in the deployment secret store. Tokens are encrypted at rest, hashed only for duplicate detection, omitted from JSON responses, and never accepted as a command argument. Share links use random expiring tokens. Temporary files used for ZIP bundling are size-capped, time-bounded, and removed after processing.

## Deployment and scaling path

Telegram long polling requires a continuously running instance. On Koyeb, configure `Minimum instances = 1`, disable Scale-to-Zero, and use `GET /healthz` as the public lightweight health check. The current manager is process-local: it is safe for one service instance, but multiple replicas require a durable queue, distributed worker leases, encrypted secret rotation, and leader election before they can safely share managed tokens.

The next production scaling step is to move outbound factory jobs behind a durable MongoDB-backed queue and reserve a worker lease per job. Broadcast delivery can similarly move to a rate-limited worker with retries. MongoDB transactions or soft-delete workflows should be introduced when content deletion or bot deletion becomes business-critical.
