# Contributing

Thank you for improving Telegram Archive Bot. Keep changes focused, secure, and easy to review.

## Development workflow

Use Go 1.23 or newer, copy `.env.example` to a local environment file, and never commit secrets. Run formatting, tests, and a build before opening a pull request:

```bash
gofmt -w .
go test ./...
go build ./...
```

## Pull requests

Describe the user-facing behavior, security implications, database changes, and any operational configuration required. Add regression tests for new handlers, services, API routes, or parsing rules. Do not add real tokens, user data, database dumps, or generated deployment secrets.

## Architecture rules

Keep Telegram handlers thin and place business logic in services. Validate authorization at the boundary and again in sensitive services. Use bounded pagination and timeouts for external or database operations. Store file metadata in MongoDB and keep temporary archive files bounded and short-lived.
