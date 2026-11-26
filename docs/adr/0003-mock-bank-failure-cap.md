# ADR 0003: Mock bank failure injection is capped per payment

Status: accepted · Date: 2026-11-26

## Context

With `MOCKBANK_FAIL_RATE=0.5` and 5 debit attempts, a payment would fail about 3% of the time (`0.5^5`). The demo and the end-to-end test must settle reliably.

## Decision

The mock bank injects at most 3 consecutive failures per idempotency key. With a 5-attempt retry policy a payment always settles, yet retries still show in the timeline and in the Temporal UI. Setting the rate to `1` makes every payment fail exactly three times, which the e2e test uses to make retries visible deterministically.
