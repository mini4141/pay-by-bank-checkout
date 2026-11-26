# 0002: Idempotency

Status: accepted · Date: 2026-11-26

## Problem

Clients retry: a shopper double-taps "Pay", a merchant's server times out and re-sends `CreateCheckout`, a worker retries an activity after a crash. None of these may create two checkouts, two payments or two debits.

## Decision: idempotency at every layer

| Layer | Mechanism |
|---|---|
| `CreateMerchant` / `CreateCheckout` | `idempotency_key` in the request. `INSERT ... ON CONFLICT DO NOTHING` on a unique key (`merchants.idempotency_key`, `checkouts (merchant_id, idempotency_key)`), then read the existing row. A replayed `CreateMerchant` cannot return the API key again (only its hash is stored). |
| `StartPayment` | One payment per checkout (`payments.checkout_id UNIQUE`). The Temporal workflow ID is `payment-<checkout_id>` with conflict policy `USE_EXISTING` and reuse policy `REJECT_DUPLICATE`; an "already started" error is treated as success. A duplicate submission therefore attaches to the same workflow. |
| `RefundPayment` | Workflow ID `refund-<payment_id>`, same policies; a second click is a no-op. Ledger rows are unique per `(payment_id, kind)`. |
| Activities | Every bank call carries the idempotency key `payment_id` (per operation); the bank replays the first result. |
| Webhooks | Event ID is `<payment_id>:<event type>` so merchants can de-duplicate; each attempt is logged. |

## Money

Amounts are `int64` minor units plus an ISO 4217 currency (JPY has 0 decimals, so ¥1,200 is `1200`). Floating point never touches money. The UI derives fraction digits from `Intl.NumberFormat`.

## Not covered

`StartPayment.idempotency_key` is accepted for API symmetry; the checkout ID is the real key (a checkout can be paid once). A merchant-scoped key store with request fingerprinting and expiry is listed in [tech-debt](../tech-debt.md).
