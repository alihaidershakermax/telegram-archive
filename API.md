# Telegram Archive Bot API

The bot exposes a versioned HTTP API from the same process as the Telegram polling worker. Set `API_KEY` to enable protected endpoints. The public health endpoint remains available without authentication.

## Authentication

Send either `X-API-Key: <API_KEY>` or `Authorization: Bearer <API_KEY>`. Protected routes are limited by `API_RATE_LIMIT` requests per minute per API key and source address. Do not expose the API directly to the public internet without TLS and a reverse proxy or private network policy.

## Health

```bash
curl http://localhost:8000/api/v1/health
```

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
