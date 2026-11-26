# PROGRESS

Resume from the first unticked item. Phases follow CLAUDE.md §5.

## Phase 0: scaffold
- [x] Repo layout, README, PROGRESS, DECISIONS, .gitignore, .gitattributes, .env.example, CI workflow

## Coverage matrix (SPECS §2)
- [x] Features end to end (UI → services → data): Checkout (Svelte) → Connect API (Go) → Temporal → Postgres, proven by e2e test
- [x] Design proposals and plans: `docs/design/0001…`, `0002…`, ADRs
- [x] Ship-fast vs redesign trade-offs, tech-debt spotting: ADR "status via table vs query", `docs/tech-debt.md`
- [x] Comfortable across the stack: Svelte + React + Go + SQL + infra
- [x] Stack used: TS/JS, SvelteKit, React, Go, ConnectRPC/Protobuf, Temporal, PostgreSQL, AWS, Terraform
- [x] Payments correctness: idempotency, minor units, workflow determinism, signed webhooks, tested

## Phase 1: local (SPECS §9)
- [x] F1 merchant onboarding + checkout creation
- [x] F2 shopper checkout (SvelteKit)
- [x] F3 payment workflow (Temporal)
- [x] F4 merchant dashboard (React)
- [x] F5 refunds
- [x] F6 design documents
- [x] `tests/`: go test JUnit, vitest, Playwright report
- [x] screenshots 01-dashboard-new-checkout, 02-checkout-bank-select, 03-checkout-consent, 04-checkout-timeline-settled, 05-dashboard-live-payment, 06-webhook-deliveries, 07-temporal-ui-history-retries, 08-refunded
- [x] logs: worker.log, mockbank-failures.log, buf-lint.txt
- [x] `scripts/test-all.sh` green, CI green on main, proof index complete

## Phase 2: AWS (SPECS §10)
- [x] screenshots 01-cloud-checkout-settled, 02-cloud-dashboard, 03-cloud-temporal-ui
- [x] logs: ecs-services.json, cloudwatch-worker.log, resources.txt
- [x] tests/playwright-cloud/

## Phase 3: destroy (SPECS §11)
- [x] `terraform destroy` done
- [x] `verify-destroyed.sh` shows nothing billable (ECS cluster, ALB, RDS, CloudFront, ECR, Cloud Map)

## Phase 4: write-up
- [x] `docs/PORTFOLIO.md`, README final, `docs/AI_USAGE.md`, everything pushed

## Definition of done (CLAUDE.md §12)
- [x] Every stack item used in running code
- [x] Phase 1 proof complete, tests green locally and in CI
- [x] Phase 2 proof complete
- [x] Phase 3 everything destroyed
- [x] PORTFOLIO.md and README written, committed, pushed
