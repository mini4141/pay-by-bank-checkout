# Proof index

Every artifact is produced by a script (`scripts/capture-proof.sh`), not by hand. Real data flows through the system in all of them.

## Phase 1: local (Docker Compose)

Run: `scripts/capture-proof.sh phase1`

| Artifact | What it proves |
|---|---|
| [screenshots/01-dashboard-new-checkout.png](phase1-local/screenshots/01-dashboard-new-checkout.png) | Merchant creates a ¥1,200 checkout in the React dashboard; URL + QR code |
| [screenshots/02-checkout-bank-select.png](phase1-local/screenshots/02-checkout-bank-select.png) | SvelteKit mobile checkout shows merchant, amount and the 3 mock banks |
| [screenshots/03-checkout-consent.png](phase1-local/screenshots/03-checkout-consent.png) | Bank consent screen inside the flow |
| [screenshots/04-checkout-timeline-settled.png](phase1-local/screenshots/04-checkout-timeline-settled.png) | Live timeline (WatchPayment stream) completed without a refresh |
| [screenshots/05-dashboard-live-payment.png](phase1-local/screenshots/05-dashboard-live-payment.png) | The payment appeared live on the dashboard, SETTLED, with 3 retried debits in its timeline |
| [screenshots/06-webhook-deliveries.png](phase1-local/screenshots/06-webhook-deliveries.png) | Signed webhook delivery recorded (HTTP 204) |
| [screenshots/07-temporal-ui-history-retries.png](phase1-local/screenshots/07-temporal-ui-history-retries.png) | Temporal UI: `PaymentWorkflow` completed, `Debit` shown with retry attempts |
| [screenshots/08-refunded.png](phase1-local/screenshots/08-refunded.png) | RefundWorkflow: status REFUNDED, balance back to ¥0, `payment.refunded` webhook |
| [tests/go-junit.xml](phase1-local/tests/go-junit.xml), [go-test.log](phase1-local/tests/go-test.log) | Go unit (Temporal testsuite: happy path, debit retried after one error) and sqlc integration tests |
| [tests/vitest-checkout.xml](phase1-local/tests/vitest-checkout.xml), [vitest-dashboard.xml](phase1-local/tests/vitest-dashboard.xml) | Svelte timeline and React payments table component tests |
| [tests/playwright/](phase1-local/tests/playwright) | Playwright e2e report (HTML + JUnit): full demo story including webhook check and refund |
| [logs/test-all.log](phase1-local/logs/test-all.log) | `scripts/test-all.sh` output, every layer green |
| [logs/buf-lint.txt](phase1-local/logs/buf-lint.txt) | `buf lint` clean on `proto/pay/v1` |
| [logs/worker.log](phase1-local/logs/worker.log) | Worker structured logs |
| [logs/mockbank-failures.log](phase1-local/logs/mockbank-failures.log) | Mock bank injected failures (`injected_failure`) and replays |
| [logs/api-receiver.log](phase1-local/logs/api-receiver.log) | API requests and signed webhooks received |

## Phase 2: AWS

Pending (see `phase2-cloud/`).

## Phase 3: destroy

Pending (see `phase3-destroy/`).
