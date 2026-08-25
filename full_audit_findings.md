# Full Code Audit Findings

Date: 2026-08-25

## Scope

The audit covered application startup and shutdown, parent and managed-bot polling, distributed leases, update routing, Reply and Inline keyboards, namespace isolation, MongoDB lifecycle, storage delivery, subscriptions, Bot Factory permissions, API authentication, and resource bounds.

## Confirmed fixes applied

| Area | Finding | Action |
|---|---|---|
| User labels | `FormatUserLabel` converted an int64 ID through a rune, producing incorrect labels for IDs above the Unicode range. | Replaced with `strconv.FormatInt` and added regression coverage. |
| Maintenance settings | Child-scoped maintenance writes updated the process-wide primary cache, allowing cross-namespace state leakage. Failed writes also changed the cache. | Cache now updates only after a successful unscoped write. |
| Migration IDs | Migration accepted only one identifier format while `/mybots` exposed another. | Resolver now accepts Telegram bot ID or managed-bot record ID. |
| Group context | Parent group handling could select the wrong database scope. | Group handling now uses the unified archive context. |
| Group upsert | `SetGroupEnabled` could create an incomplete record and later fail decoding the ID. | Upsert now creates a complete record with a string ID and timestamps. |
| Upload picker | Delayed category picker used the wrong destination for group uploads. | Picker now returns to the originating chat. |
| Subscriber notifications | Find-then-send-then-insert could deliver duplicates under concurrent workers. | Atomic claim is made before sending; failed sends remove the claim for retry. |
| Worker lifecycle | Poll exit on a closed updates channel did not always stop health/lease goroutines or remove the worker. | Poll now performs idempotent shutdown and map removal on every exit path. |
| Telegram HTTP | `NewBotAPI` used an `http.Client` without a timeout. | Parent and child clients now use a bounded 45-second client timeout. |
| Mongo indexes | Index creation could use `context.Background()` indefinitely and delay or hang startup. | `EnsureIndexesForContext` now applies a 60-second timeout when the caller has no deadline. |
| API permissions | Scoped keys were checked only as generic read/write; documented `bot:settings` and `archive:delete` semantics were not enforced. | Route mapping now distinguishes group settings, archive reads, writes, and deletes. |
| Callback acknowledgement | Callback acknowledgement occurred after slow quota/database work. | Acknowledgement is sent at the beginning of update handling. |
| Reply Keyboard | Legacy text buttons were ignored when no awaiting workflow existed. | Compatibility mapping now routes old keyboard labels to current actions. |

## Important risks still requiring operational or design work

1. Koyeb `SIGTERM` is external to the application. The application does not implement scale-to-zero. Long polling requires a hosting configuration that keeps at least one instance alive.

2. When a process cannot acquire the parent lease at startup, the current design intentionally keeps the parent update stream idle. A standby instance does not yet retry acquisition after the current lease owner disappears. This can leave the parent bot unavailable until the standby is restarted. This is a high-priority failover improvement.

3. `RestoreBotBackup` is destructive and non-transactional: it clears target collections before inserting the snapshot. A mid-restore failure can leave a partial namespace. Restore should be staged or protected by a transaction/rollback strategy before being treated as fully production-safe.

4. `autoFinishUpload` uses a five-second sleep per upload session and is not cancellation-aware. It is bounded and lightweight, but shutdown can leave a short-lived goroutine that attempts a best-effort send after shutdown.

5. `bot:analytics` is advertised for scoped API keys, but no scoped analytics route currently consumes that permission. It should be documented as reserved until an analytics route is exposed.

## Verification

The latest local verification passed:

```text
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

Production verification still requires a deployment and a live `/start` plus keyboard press while the instance is running. The supplied Koyeb logs did not contain a Telegram update, so they could not validate keyboard dispatch.
