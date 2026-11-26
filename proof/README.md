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

## Phase 2: AWS (ECS Fargate, ALB, RDS, ECR, S3; stack was up for under an hour)

Run: `scripts/deploy.sh` (which calls `scripts/capture-cloud-proof.sh`). Dashboard is served from S3 website hosting because AWS had not verified the account for CloudFront ([ADR 0002](../docs/adr/0002-cloudfront-fronts-api.md)).

| Artifact | What it proves |
|---|---|
| [screenshots/01-cloud-checkout-settled.png](phase2-cloud/screenshots/01-cloud-checkout-settled.png) | Shopper paid through the ALB-hosted SvelteKit checkout; timeline settled |
| [screenshots/02-cloud-dashboard.png](phase2-cloud/screenshots/02-cloud-dashboard.png) | The S3-hosted React dashboard shows the live payment, timeline and webhook delivery |
| [screenshots/03-cloud-temporal-ui.png](phase2-cloud/screenshots/03-cloud-temporal-ui.png) | Temporal UI on the ALB (:8233, deployer IP only) with the completed `PaymentWorkflow` |
| [screenshots/05-cloud-temporal-retried-debit.png](phase2-cloud/screenshots/05-cloud-temporal-retried-debit.png) | A cloud payment whose `Debit` was retried 3 times (50% bank failure rate) and still settled |
| [screenshots/04-cloud-alb-request-count.png](phase2-cloud/screenshots/04-cloud-alb-request-count.png) | CloudWatch metric graph rendered from the CLI: ALB request count during the run |
| [screenshots/cloud-01 ... cloud-08](phase2-cloud/screenshots) | Every step of the demo story in the cloud, including the refund |
| [tests/playwright-cloud/](phase2-cloud/tests/playwright-cloud) | Playwright report (HTML + JUnit) for the story run against the cloud endpoints |
| [logs/ecs-services.json](phase2-cloud/logs/ecs-services.json) | `describe-services`: all six services ACTIVE, running = desired |
| [logs/cloudwatch-worker.log](phase2-cloud/logs/cloudwatch-worker.log), [cloudwatch-mockbank.log](phase2-cloud/logs/cloudwatch-mockbank.log) | Worker and mock bank logs from CloudWatch, including injected debit failures |
| [logs/playwright-cloud.log](phase2-cloud/logs/playwright-cloud.log) | Test run output |
| [resources.txt](phase2-cloud/resources.txt) | Terraform outputs, tagged resources, RDS and dashboard hosting details |

## Phase 3: destroy

| Artifact | What it proves |
|---|---|
| [phase3-destroy/verify-destroyed.txt](phase3-destroy/verify-destroyed.txt) | After `scripts/destroy.sh` (63 resources destroyed), no ECS cluster, ALB, RDS, CloudFront, ECR, Cloud Map, S3, NAT, VPC, log groups or SSM parameters remain. The tagging API still lists a few ECS objects for a few minutes; the direct checks confirm they are gone. |
