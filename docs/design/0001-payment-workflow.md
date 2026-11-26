# 0001: Payment workflow

Status: accepted · Date: 2026-11-26

## Problem

A shopper pays a merchant straight from their bank account (account-to-account). The money movement spans several systems that each fail independently: the bank (reserve, debit), our database (ledger), and the merchant (webhook). The payment must **finish correctly or compensate**, show **live progress** to the shopper and merchant, and survive process restarts without double-charging.

## Constraints

- Correctness over throughput: money is `int64` minor units, steps are idempotent, no double debit.
- The bank is flaky (simulated by `MOCKBANK_FAIL_RATE`); transient failures must be invisible to the shopper.
- Small team, early-stage product: prefer fewer moving parts.
- Observable: an operator must be able to answer "where is payment X stuck?" in one click.

## Options

| Option | Pros | Cons |
|---|---|---|
| **A. Synchronous API call chain** (handler calls bank, writes ledger, calls webhook) | Simplest | A crash between debit and ledger loses money; retries are hand-rolled; a long request holds the client |
| **B. Queue + worker + state table** | Decoupled, familiar | Retries, timers, compensation and visibility are re-implemented by us; state machine bugs are likely |
| **C. Temporal workflow** | Durable execution, per-activity retries/timeouts, signals, queries, full history UI | Another service to run; determinism rules for workflow code |

## Decision

**Option C.** `PaymentWorkflow` (ID `payment-<checkout_id>`) runs these activities:

1. `ReserveFunds`: bank consent + reserve (idempotent by `payment_id`).
2. `RiskCheck`: rule `amount <= RISK_LIMIT_MINOR`. If rejected: `ReleaseReservation` (compensation), mark `FAILED`.
3. An optional `cancel` signal is honoured only before the debit.
4. `Debit`: retry policy initial 1 s, backoff x2, max 5 attempts. On final failure: release the reservation, mark `FAILED`.
5. `SettleToMerchant`: one DB transaction writes the ledger row, the `SETTLED` status and the event.
6. `DeliverWebhook`: HMAC-signed, up to 10 attempts; each attempt is stored in `webhook_deliveries`.

`RefundWorkflow` (`refund-<payment_id>`) credits the shopper's bank, writes a negative ledger row, sets `REFUNDED` and delivers `payment.refunded`.

Workflow code is deterministic (no clock, randomness or I/O). All I/O lives in activities, which are idempotent: the mock bank replays the first result per idempotency key, `ledger_entries` is unique on `(payment_id, kind)`, and status writes are plain overwrites.

A query handler `status` exposes the current step; the Temporal UI shows the full history including activity retries.

## Live status

Activities write `payment_events` rows and bump `payments.updated_at` in the same transaction. The `WatchPayment` and `WatchMerchantPayments` streaming RPCs poll that table every 250 ms and push a snapshot when it changes. See [ADR 0001](../adr/0001-status-via-table.md).

## Failure modes

| Failure | Behaviour |
|---|---|
| Bank returns 503 on debit | Activity retried (1 s, 2 s, 4 s, ...); each failed attempt is recorded as a `RETRY` event so the shopper sees "retrying" |
| Bank down for all 5 attempts | Reservation released, payment `FAILED`, merchant gets `payment.failed` |
| Worker crashes mid-payment | Temporal replays history on another worker; completed activities are not re-run; the in-flight one is retried and is idempotent |
| API crashes after inserting the payment but before starting the workflow | The shopper's retry hits the unique `checkout_id`, re-attaches to the payment and starts the workflow (`USE_EXISTING`) |
| Merchant endpoint down | Webhook retried up to 10 times; the payment stays `SETTLED`; deliveries show the failures |
| Duplicate submit | Same workflow ID: attaches to the existing execution ([0002](0002-idempotency.md)) |

## Rollout

1. Run the worker next to the API with the Temporal dev server (done: compose and Fargate).
2. Production would use a persistent Temporal cluster (Temporal Cloud or self-hosted on Postgres) and add worker versioning before changing workflow code.
