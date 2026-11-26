# ADR 0001: Propagate payment status via a status table, not workflow queries

Status: accepted · Date: 2026-11-26

## Context

`WatchPayment` and `WatchMerchantPayments` must push state transitions. The API can either (a) query the Temporal workflow (`status` query) or (b) read a status table that activities update.

## Decision

Use the table (`payment_events` plus `payments.status/updated_at`), written by the activities in the same transaction as the business change. The streaming RPCs poll it every 250 ms and send a snapshot when `updated_at` or the event/delivery counts change.

## Why

- **Ship fast:** one SQL read model serves the shopper stream, the dashboard list, payment detail and webhook history. No extra Temporal round trip per watcher.
- **Works after the workflow closes:** queries on completed workflows depend on retention; the table is the system of record the UI needs anyway.
- **Atomic:** event + ledger + status commit together, so a snapshot never shows `SETTLED` without the ledger row.
- A workflow `status` query still exists for operators (Temporal UI, `temporal workflow query`).

## Trade-offs

- Polling adds up to 250 ms latency and one cheap query per open stream. Fine at this scale; the next step is Postgres `LISTEN/NOTIFY` (or an outbox). Logged in [tech-debt](../tech-debt.md).
- Two sources of truth (workflow history and table) can diverge if an activity fails after the DB write; activities are idempotent so a retry converges.
