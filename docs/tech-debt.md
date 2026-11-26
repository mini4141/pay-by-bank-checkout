# Tech-debt register

Conscious shortcuts taken to ship a breadth-first showcase.

| # | Shortcut | Risk | Next step |
|---|---|---|---|
| 1 | Streams poll Postgres every 250 ms ([ADR 0001](adr/0001-status-via-table.md)) | Load grows with watchers | `LISTEN/NOTIFY` or outbox + fan-out |
| 2 | `CreateMerchant` is unauthenticated | Anyone can create merchants | Admin credential or signup flow |
| 3 | API keys are a single SHA-256, no scopes or rotation | A key leak is all-powerful | Scoped, rotatable keys |
| 4 | One payment per checkout; no retry after `FAILED` | The shopper needs a new checkout | Allow a new attempt keyed by attempt number |
| 5 | Idempotency keys on mutating RPCs are only enforced where a natural key exists | Weak once partial refunds exist | Key store with request fingerprints and TTL |
| 6 | Temporal dev server (single node, no auth) | No HA, data lost on restart | Temporal Cloud or clustered self-hosting with mTLS |
| 7 | Mock bank state is in memory | Lost on restart; fine for a mock | None needed |
| 8 | The merchant list loads all events/deliveries for the last 50 payments on each change | Wasteful as payments grow | Fetch detail on selection; pagination |
| 9 | The consent screen is rendered by the checkout app instead of redirecting to a bank | Not a real consent redirect | Redirect with a signed consent token and callback |
| 10 | The dashboard keeps the API key in `localStorage` | XSS exposure | Session cookie via a BFF |
| 11 | ECS tasks run in public subnets with public IPs (no NAT) to save cost | Larger attack surface | Private subnets + VPC endpoints/NAT in production |
| 12 | Webhook secrets are stored in plaintext in Postgres | A DB leak exposes secrets | Envelope encryption (KMS) |
| 13 | The webhook receiver keeps deliveries in memory | Lost on restart | Persist if it ever matters |
