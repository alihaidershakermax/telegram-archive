# Platform References

## Telegram Bot API

- Telegram Bot API: https://core.telegram.org/bots/api
  - Telegram documents two mutually exclusive update delivery methods: `getUpdates` and webhooks.
  - Bot tokens authenticate Bot API requests and must be kept secret.
  - The Bot API includes methods for managed-bot access in newer versions; support and permissions must be verified before implementing automatic bot creation.

- Telegram Bot Features: https://core.telegram.org/bots/features
  - Telegram describes bot management and managed-bot capabilities separately from ordinary BotFather-created bot tokens.
  - The current implementation uses the compatible, explicit-token onboarding route: the operator creates a bot in BotFather and registers its token with the factory.

## Koyeb

- Scale-to-Zero: https://www.koyeb.com/docs/run-and-scale/scale-to-zero
  - Koyeb can scale a service to zero when its minimum instance count is zero and the service receives no qualifying internet traffic.
  - The Koyeb Free Instance can scale down after an idle period; a Telegram long-polling worker should therefore use at least one permanent instance.

- Health Checks: https://www.koyeb.com/docs/run-and-scale/health-checks
  - Koyeb uses health checks at startup and for liveness.
  - Custom HTTP health checks must return a 2xx or 3xx status; this project exposes `GET /healthz` without database dependency.

These references were reviewed on 2026-08-22. They are platform documentation, not a substitute for verifying the live Telegram account permissions or the deployed Koyeb service configuration.
