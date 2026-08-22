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

## Deployment checklist

On Koyeb, use `Minimum instances = 1`, disable Scale-to-Zero, expose the service `PORT`, and configure `GET /healthz` as the public HTTP health check. Set the startup grace period long enough for MongoDB and Telegram initialization. Keep `BOT_TOKEN`, `MONGO_URI`, `API_KEY`, `AI_API_KEY`, and `FACTORY_ENCRYPTION_KEY` in the secret store. Do not paste production tokens into issues, pull requests, or shell history.
