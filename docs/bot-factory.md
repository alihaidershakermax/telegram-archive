# Bot Factory

## Purpose

Bot Factory is an operator-controlled service for managing multiple Telegram bots from one controller bot. The first release uses the compatible onboarding model: an authorized operator creates a bot in Telegram BotFather and submits its token to the controller. Automatic Managed Bot creation is deliberately not assumed because it depends on Telegram account permissions and Bot API support.

## Safe onboarding

Use `/newbot` in the controller bot. The service asks for the token in a separate message, validates it with Telegram `getMe`, encrypts it with AES-256-GCM, calculates a SHA-256 duplicate-check hash, and stores the encrypted record in MongoDB. The plaintext token is not written to logs, returned by API responses, or accepted as a command argument. `/mybots` shows safe metadata and `/cancel` clears an unfinished flow.

The required `FACTORY_ENCRYPTION_KEY` must decode to exactly 32 bytes. It can be a 64-character hexadecimal value or a base64-encoded 32-byte value. Store it only in the deployment secret manager. Losing this key makes existing encrypted registrations unrecoverable, so keep an offline operational backup according to the deployment security policy.

## API v2 surface

| Endpoint | Description |
|---|---|
| `GET /api/v2/health` | Public configuration health for the factory |
| `GET /api/v2/bots` | Safe list of managed bots and live metrics |
| `POST /api/v2/bots` | Register a BotFather token |
| `GET /api/v2/bots/{id}` | Read one safe managed-bot record |
| `POST /api/v2/bots/{id}/pause` | Stop polling and preserve registration |
| `POST /api/v2/bots/{id}/resume` | Decrypt, revalidate, and restart polling |
| `DELETE /api/v2/bots/{id}` | Stop and permanently delete the registration |
| `GET /api/v2/router/best` | Select the healthiest worker |
| `POST /api/v2/router/send` | Send a text through the selected worker |

All protected endpoints require the existing `X-API-Key` or bearer token. API v2 uses the configured platform owner for registrations; the request body cannot choose another owner. For external multi-tenant use, introduce scoped API keys and an authenticated tenant table before exposing the service beyond trusted operators.

## Routing policy

The manager keeps one long-polling stream per bot token. It records active requests, last observed processing latency, health recency, consecutive failures, and accumulated errors. The best-worker score prefers a recent, low-latency, low-error, low-load worker. A circuit breaker excludes a worker after repeated health failures. Telegram updates cannot be moved between tokens, so this policy routes new outbound work and never merges independent Telegram sessions.

The current manager is process-local and designed for one Koyeb instance. Multiple replicas require a durable job queue, a distributed lease per bot token, leader election or webhook partitioning, and a secret-rotation procedure before horizontal scaling.

## Data isolation and shared storage

Each managed bot receives a stable MongoDB database named `archive_bot_{telegram_bot_id}` and a human-readable logical folder such as `bots/archive_bot_123456/`. Categories, subjects, files, users, permissions, settings, ratings, share links, logs, and counters are read and written from that bot database when an update is handled by the managed worker. The primary controller continues to use the configured legacy database, so existing archive data remains available without an unsafe destructive migration.

The Telegram storage channel has one owner: the primary controller bot. Managed bots do not need to be channel administrators. When a managed bot receives an upload, the service downloads the media through that bot, re-uploads it to the shared channel using the primary bot, and stores the file ID returned for the primary bot. When a user downloads or shares a file from a managed bot, the primary bot sends the media because Telegram `file_id` values are scoped to the bot that received them. Share links remain on the originating bot so their scoped share record is resolved in the correct database.

This arrangement provides logical and operational isolation: a worker cannot query another worker's database through normal handler paths, and no child token is granted channel access. The primary bot must be able to post to the shared channel, and a user must have initiated the primary bot before the primary bot can deliver a file directly to that user. If direct delivery through the primary bot is unavailable, the service records the send failure rather than incorrectly retrying with a child bot using an incompatible file ID.

## Deployment checklist

On Koyeb, use `Minimum instances = 1`, disable Scale-to-Zero, expose the service `PORT`, and configure `GET /healthz` as the public HTTP health check. Set the startup grace period long enough for MongoDB and Telegram initialization. Keep `BOT_TOKEN`, `MONGO_URI`, `API_KEY`, `AI_API_KEY`, and `FACTORY_ENCRYPTION_KEY` in the secret store. Do not paste production tokens into issues, pull requests, or shell history.
