# Telegram Archive Bot API

The bot exposes a versioned HTTP API from the same process as the Telegram polling worker. Set `API_KEY` to enable protected endpoints. The public health endpoint remains available without authentication.

## Authentication

Send either `X-API-Key: <API_KEY>` or `Authorization: Bearer <API_KEY>`. Protected routes are limited by `API_RATE_LIMIT` requests per minute per API key and source address. Do not expose the API directly to the public internet without TLS and a reverse proxy or private network policy.

## Health

```bash
curl http://localhost:8000/api/v1/health
curl http://localhost:8000/healthz
```

Use `GET /healthz` as the Koyeb HTTP health-check path. It returns `200 OK` without API-key authentication and does not wait for MongoDB or Telegram, so platform liveness checks remain fast while the bot continues its polling worker.

## Archive data

```bash
curl -H "X-API-Key: $API_KEY" \
  http://localhost:8000/api/v1/categories

curl -H "X-API-Key: $API_KEY" \
  "http://localhost:8000/api/v1/subjects?category_id=1"

curl -H "X-API-Key: $API_KEY" \
  "http://localhost:8000/api/v1/files?subject_id=1"
```

Responses use the following envelope:

```json
{"data": []}
```

## Bot-specific archive namespaces

The archive endpoints use the primary database by default. To read a managed bot archive, send the registered Telegram bot ID in `X-Telegram-Bot-ID`; the server verifies that the bot exists before selecting its isolated MongoDB database. The same header applies to `/api/v1/categories`, `/api/v1/subjects`, `/api/v1/files`, and `/api/v1/bundle`.

```bash
curl -H "X-API-Key: $API_KEY" \
  -H "X-Telegram-Bot-ID: 123456789" \
  http://localhost:8000/api/v1/categories
```

The primary bot is the sole Storage Gateway for the shared Telegram channel. Managed bots do not need to be channel administrators. Uploads received by a managed bot are transferred through that bot and re-uploaded by the primary bot; the database stores the primary bot's returned `file_id`. Downloads and shared-file delivery likewise use the primary bot, because Telegram file IDs are bot-specific.

## AI Gateway

The gateway forwards requests to an OpenAI-compatible provider configured with `AI_BASE_URL`, `AI_API_KEY`, and `AI_MODEL`. The server never exposes the upstream provider key to callers.

### Chat

```bash
curl -X POST http://localhost:8000/api/v1/ai/chat \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "messages": [
      {"role":"system","content":"You are an educational assistant."},
      {"role":"user","content":"اشرح قانون نيوتن الثاني باختصار"}
    ],
    "max_tokens": 800
  }'
```

### Summarization

```bash
curl -X POST http://localhost:8000/api/v1/ai/summarize \
  -H "X-API-Key: $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "text":"النص الأكاديمي المراد تلخيصه...",
    "language":"Arabic",
    "style":"concise educational"
  }'
```

The summarization endpoint accepts up to 100,000 UTF-8 bytes per request and returns the provider-compatible chat response. A missing AI configuration returns `503`; upstream failures return `502` without exposing provider details.

## Recommended deployment

Keep `API_KEY`, `AI_API_KEY`, and `BOT_TOKEN` in the deployment secret store. Place the service behind HTTPS, restrict the API port to trusted clients, rotate keys periodically, and add a separate gateway or reverse proxy if multiple consumers need different quotas or scopes. The current first version uses one application key; scoped keys, JWT/OAuth, usage accounting, and a persistent job queue should be added before opening the API to third parties.

## Bot Factory API v2

API v2 manages bots that have already been created through Telegram BotFather. The factory validates each supplied token with `getMe`, encrypts the token using AES-256-GCM, stores only safe metadata in responses, and starts one isolated polling worker per active bot. The API key identifies the platform operator; the token owner is always the configured `OWNER_ID` and is never accepted from a request body.

Before enabling the factory, set `FACTORY_ENCRYPTION_KEY` to a random 32-byte value encoded as 64 hexadecimal characters or base64. Set `FACTORY_MAX_BOTS_PER_OWNER` to the maximum registration count and keep `FACTORY_WORKERS` ready for future queued outbound jobs. The factory remains disabled when the encryption key is absent.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/v2/health` | Reports whether the factory is configured; public and cache-free |
| `GET` | `/api/v2/bots` | Lists safe metadata and live metrics for managed bots |
| `POST` | `/api/v2/bots` | Registers a BotFather token and starts its worker |
| `GET` | `/api/v2/bots/{id}` | Returns one managed bot without token material |
| `POST` | `/api/v2/bots/{id}/pause` | Stops polling while preserving encrypted registration |
| `POST` | `/api/v2/bots/{id}/resume` | Revalidates the encrypted token and restarts polling |
| `DELETE` | `/api/v2/bots/{id}` | Stops the worker and permanently deletes the registration |
| `GET` | `/api/v2/router/best` | Selects the healthiest available bot using live routing signals |

```bash
curl -H "X-API-Key: $API_KEY" http://localhost:8000/api/v2/bots

curl -X POST http://localhost:8000/api/v2/bots \\
  -H "X-API-Key: $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"token":"123456:REDACTED"}'

curl -X POST -H "X-API-Key: $API_KEY" http://localhost:8000/api/v2/bots/<id>/pause
curl -X POST -H "X-API-Key: $API_KEY" http://localhost:8000/api/v2/bots/<id>/resume
curl -H "X-API-Key: $API_KEY" http://localhost:8000/api/v2/router/best
curl -X DELETE -H "X-API-Key: $API_KEY" http://localhost:8000/api/v2/bots/<id>
```

The intelligent routing score uses health recency, observed latency, active requests, consecutive errors, and accumulated errors. Telegram update streams cannot be merged across tokens, so the selector chooses a healthy bot for factory-managed outbound work rather than moving one Telegram session between tokens. All v2 responses intentionally omit token ciphertext, nonce, hash, and plaintext.

## Bot Factory Telegram commands

Authorized operators can use `/newbot` to begin a two-step onboarding flow and send the token in a separate message. The token is never accepted as a command argument, which reduces accidental exposure in chat history. `/mybots` displays the caller's safe bot metadata, and `/cancel` clears a pending onboarding flow.
