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

## Production expansion

Each managed bot receives independent limits for users, files, bytes, and requests per minute. New limits can be set with `POST /api/v2/bots/{id}/limits`; usage is exposed through `/usage` and the operator monitor endpoint. The primary bot's shared Telegram channel remains the only storage backend.

File delivery is durable. Immediate delivery uses the primary bot; failures become `storage_jobs` records and are retried with exponential backoff. Jobs that exceed the configured attempts become `dead`, while stale processing jobs are reclaimed after a process restart.

The factory supports `owner`, `admin`, `editor`, and `viewer` roles in each bot database. Management actions are written to audit logs. Maintenance mode is scoped to the bot database, so pausing one archive does not pause another.

Operators can rotate a token only when the replacement belongs to the same Telegram bot identity. The namespace is never changed during rotation. Backups and restores are scoped by both Telegram bot ID and backup ID; factory registration metadata and other bot databases are not included in a restore.

The master API key can create bot-scoped API keys. A scoped key must be paired with the same `X-Telegram-Bot-ID` header and is limited to archive/AI paths with explicit permissions. The raw secret is returned once and is never stored; only a SHA-256 hash is persisted.

AI indexing accepts text for a file that belongs to the selected bot, asks the configured provider for a summary and tags, and stores the resulting `ai_indexes` record inside that bot's database. AI request counts are recorded in the same namespace.

## Parent-controlled database auto-scaling

The parent bot starts an `AutoExpansionController` when `DB_AUTO_EXPANSION=true`. The controller scans active child bots, derives each bot database from its Telegram bot ID, and records the latest counts for users, files, and file bytes in the primary `expansion_states` collection. It does not share archive records between databases.

Each scan uses a MongoDB lease keyed by the child bot ID. A second parent process cannot expand the same child database while the lease is active, and the cooldown prevents continuous rescans after a successful pass. If a process crashes, the lease expires and a later scan can safely resume.

Schema preparation is additive and idempotent. The controller ensures indexes in the child database and records `001_base_indexes` in `schema_migrations`; it never drops, renames, or rewrites live collections. Capacity is reported as `standard` or `expanded` according to the configured document and byte thresholds. Actual disk-tier growth remains the responsibility of the MongoDB provider or Atlas autoscaling policy; the parent controller handles per-bot provisioning, indexes, migrations, and capacity signals without stopping the Telegram workers.

The latest state is available at `GET /api/v2/bots/{id}/expansion`. Configure the scan with `DB_EXPANSION_POLL_SECONDS`, `DB_EXPANSION_BATCH_SIZE`, `DB_EXPANSION_MAX_DOCS`, `DB_EXPANSION_MAX_BYTES`, `DB_EXPANSION_LOCK_SECONDS`, and `DB_EXPANSION_COOLDOWN_SECONDS`.

## MongoDB cluster control from the parent bot

The parent bot now provides `/dbpanel`, `/adddb`, and `/dbs`. The owner starts `/adddb` in a private chat, supplies a short cluster name, and then sends the MongoDB URI in a separate message. The URI is deleted immediately after receipt, verified with a short-lived ping connection, encrypted with the existing factory key, and never returned in API responses or logs.

A newly registered managed bot is assigned to the active cluster with the fewest current bot assignments. Existing bots keep their current `cluster_id`; adding a cluster never points an existing bot at an empty database. Moving an existing namespace requires the online migration workflow with verification and cutover, which is intentionally separate to prevent data loss.

Separate cluster registration requires MongoDB network access from the bot host, a database user with permissions for the bot databases, and TLS-enabled connection strings where supported. The primary bot database remains the control plane and stores only encrypted cluster metadata, routes, and migration state.

## Near-zero downtime namespace migration

The parent bot supports `/migratebot <bot_id> <target_cluster_id>` and `/migrationstatus [bot_id]`. Migration copies the bot database collection by collection in bounded cursor batches, upserts documents into the target, and records progress and a SHA-256 checksum in `namespace_migrations`. The worker is stopped only for the final consistency pass and route cutover. The source cluster is retained as the rollback copy and is never deleted automatically.

If any copy, checksum, route update, or worker restart fails, the migration is marked failed and the old route remains authoritative. Operators should not remove the source cluster until the target has been observed in production and a separate retention decision has been made.
