
## Official Koyeb documentation

Koyeb documents that an Instance stop sends `SIGTERM` to the container and waits for a graceful shutdown period before a forced stop. The scale-to-zero documentation says a Service can be scaled to zero when it receives no Internet traffic and that Koyeb Free Instance automatically scales to zero after one hour of no traffic; this cannot be disabled on the Free Instance.

Sources:
- https://www.koyeb.com/docs/reference/instances
- https://www.koyeb.com/docs/run-and-scale/scale-to-zero

## Dashboard access

On 2026-08-24, opening `https://app.koyeb.com/` redirected to the Koyeb sign-in page. The browser session is not authenticated, so the scaling setting cannot be inspected or changed without the user completing Koyeb login.
