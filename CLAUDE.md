# CLAUDE.md — pay-by-bank-checkout

These are the engineering rules for Claude Code on this repository. You work **fully autonomously**. Don't stop to ask the user questions. When something is ambiguous, pick the simplest option that keeps every stack item in §2 covered. Record it in `docs/DECISIONS.md` (one line: date, decision, reason) and keep going. Read `SPECS.md` end to end before writing any code. It is the source of truth for features, architecture and proof.

## 1. Project at a glance

| | |
|---|---|
| Project | Pay-by-Bank Checkout: an end-to-end payment flow with SvelteKit, React, Go, ConnectRPC and Temporal |
| Repo | public GitHub repo `pay-by-bank-checkout`. `origin` is already configured and the default branch is `main` |
| Target role | Full-Stack Developer, early-stage fintech (TypeScript/SvelteKit, Go, Temporal, PostgreSQL) |
| Pitch | A merchant creates a checkout, the shopper pays from their (mock) bank account in a SvelteKit checkout, a Temporal workflow runs the payment end to end, and the merchant sees it settle live in a React dashboard. It's all typed end to end with Protobuf and ConnectRPC. |
| Priority | **Breadth of the tech stack beats depth of business logic.** Every technology in §2 must appear in running code and be visible in the proof. Full validation, edge cases and complete domain rules are out of scope. |

## 2. Mandatory tech stack

| Area | Technology |
|---|---|
| Frontend | TypeScript, **SvelteKit 2 + Svelte 5** (consumer checkout, `adapter-node`), **React 19** + Vite (merchant dashboard), `@connectrpc/connect-web` generated clients, Playwright, Vitest |
| Backend | **Go 1.25**, **ConnectRPC** (`connectrpc.com/connect`) + **Protobuf** managed with `buf` (lint, breaking, generate for Go + TS), server-streaming RPC for live status |
| Workflows | **Temporal** (Go SDK): `PaymentWorkflow`, `RefundWorkflow`, activities with retries, signals, queries; Temporal dev server locally and in AWS |
| Database | **PostgreSQL 17**, `pgx` + **sqlc**, `goose` migrations |
| Mock bank | Go service simulating an account-to-account bank API (consent, reserve, debit) with a failure-injection knob |
| Webhooks | HMAC-signed merchant webhooks delivered by a Temporal activity with retries; a demo merchant receiver |
| AWS | ECS Fargate (api, worker, mockbank, temporal, checkout-web, merchant-receiver), ALB (path routing), RDS PostgreSQL, ECR, CloudWatch Logs, S3 + CloudFront (merchant dashboard SPA) |
| IaC / CI | **Terraform** · GitHub Actions (buf lint/breaking, go test, svelte-check, vitest, build) |
| Docs | Design proposals (`docs/design/0001-payment-workflow.md`, `0002-idempotency.md`) and ADRs in a lightweight RFC format |

Don't swap an item for an alternative. If a library is unavailable or broken, use the closest equivalent and log the swap in `docs/DECISIONS.md`.

## 3. Environment

- The host is a **Windows 11 PC**. Claude Code's shell is Git Bash, and the container runtime is Docker Desktop (WSL2 backend). Write automation as POSIX `bash` scripts under `scripts/` and use forward-slash paths. Add a `.gitattributes` with `* text=auto eol=lf` so scripts keep LF line endings.
- Prerequisites: Go 1.25, buf, Node.js 24 + pnpm, Docker Desktop, Temporal CLI (`temporal server start-dev`, or the `temporalio/temporal` image), Terraform, AWS CLI v2. If one is missing, install it with `winget` or the official installer and record the version in `docs/DECISIONS.md`.
- The `gh` CLI is logged in to the owner's GitHub account, and AWS CLI v2 is installed.
- Don't use Bitnami charts or images (their free catalogue has been retired). Use official upstream images and charts.

## 4. AWS account rules

- Profile: **`pay-by-bank-checkout-deployer`**. Always pass it explicitly, either as `--profile pay-by-bank-checkout-deployer` or by exporting `AWS_PROFILE=pay-by-bank-checkout-deployer` inside the scripts. Never use the default profile or any other profile.
- Region: **`ap-southeast-2`** for everything except global services such as CloudFront and IAM.
- The profile has `PowerUserAccess` + `IAMFullAccess`. Start Phase 2 with `aws sts get-caller-identity --profile pay-by-bank-checkout-deployer`. If that fails, stop and write the reason at the top of `PROGRESS.md`.
- Use AWS only. Where a requirement names Azure, GCP or another hyperscaler, use the AWS equivalent stated in `SPECS.md`.
- IaC: Terraform (`infra/`). Don't create resources by hand in the console: every cloud resource comes from code in `infra/`. Keep IaC state local and gitignored.
- Tag every resource with `Project=pay-by-bank-checkout`, `Ephemeral=true`, `ManagedBy=<tool>` and `Owner=alvieirvansyah`.
- Deployment runs **from this PC via scripts** (`scripts/deploy.sh`, `scripts/destroy.sh`). GitHub Actions only builds and tests, and never holds AWS credentials.

## 5. Phases and gates

Work strictly in order. A phase is done only when its proof exists (§8) and its items are ticked in `PROGRESS.md`. Commit and push at the end of every phase.

1. **Phase 0, scaffold.** Set up the repo layout from `SPECS.md` §4. Create `README.md`, `PROGRESS.md` (copy every checklist item from `SPECS.md` §2 and §9–§11 into it), `docs/DECISIONS.md`, `.gitignore`, `.gitattributes`, `.env.example` and the CI workflow.
2. **Phase 1, local.** Build every feature in `SPECS.md` §5. Everything runs on this PC with Docker and local tooling, and makes **no AWS calls**: use the local stand-ins listed in `SPECS.md`. Run the full test suite, then capture proof into `proof/phase1-local/`. Phase 1 counts as "ready for next" only when `scripts/test-all.sh` is green, the CI run on `main` is green, and the proof index is complete.
3. **Phase 2, AWS showcase.** Deploy with `scripts/deploy.sh`, run the smoke/e2e tests against the cloud endpoints and capture proof into `proof/phase2-cloud/`. Keep the stack alive only as long as capturing takes, with a target of **under 2 hours**.
4. **Phase 3, destroy.** Right after capturing, run `scripts/destroy.sh` and then `scripts/verify-destroyed.sh`, and save the output to `proof/phase3-destroy/`. This phase is **mandatory even if Phase 2 partly failed**. Never end a session with cloud resources running.
5. **Phase 4, write-up.** Write `docs/PORTFOLIO.md`. It needs the problem, an architecture diagram (Mermaid), a stack → requirement table, the key screenshots (relative links into `proof/`), what was hard, the approximate AWS cost and how to reproduce. This file becomes a technical page on alvieirvansyah.com.

## 6. Engineering rules

- Keep it simple and conventional: idiomatic layout for each language, small modules, no premature abstraction, no dead code.
- Configure through environment variables, with a committed `.env.example`. Real `.env` files are gitignored.
- **Never commit secrets.** That includes credentials, IaC state, `.terraform/`, `cdk.out/`, kubeconfigs, private keys and certificates. Generate local dev keys and certs into a gitignored `.local/` folder.
- Every HTTP service exposes a health endpoint (`/healthz` or the framework idiom) and logs structured JSON to stdout.
- Pin versions: commit lockfiles and pin container images to at least a minor tag.
- Use one command for each job:
  - `scripts/dev-up.sh` starts everything locally and `scripts/dev-down.sh` stops it.
  - `scripts/seed.sh` loads demo data. Start-up may run it automatically so that demos never start empty.
  - `scripts/test-all.sh` runs every test layer.
  - `scripts/capture-proof.sh <phase>` captures the proof for a phase.
- Keep `README.md` current. It covers what the project is, the architecture diagram, a quickstart, deploy/destroy, a link to `proof/README.md` and `docs/PORTFOLIO.md`, and the CI badge.
- Don't put the hiring company's name anywhere in the repo. Describe the target role generically.
- `docs/AI_USAGE.md`: briefly note how AI coding tools were used and how their output was reviewed and tested.

## 7. Testing rules

- Tests are **shallow by design**: one happy-path test per feature or flow is enough. Don't write exhaustive validation or edge-case tests.
- Required layers: Go unit (Temporal `testsuite` for the PaymentWorkflow happy path + one activity retry), Go integration (sqlc queries against compose Postgres), Vitest (one Svelte component + one React component), Playwright e2e (merchant creates checkout → shopper pays → dashboard shows SETTLED + webhook received).
- `scripts/test-all.sh` runs every local layer and exits non-zero on failure. CI runs every layer that doesn't need the cloud.
- Save test output as files (JUnit XML, an HTML report or a plain log) under the current phase's `proof/.../tests/`.

## 8. Proof rules

Proof is the deliverable. Use this folder layout:

```
proof/
  README.md                 # index: each artifact on one line, with what it proves
  phase1-local/
    screenshots/            # PNG, named NN-what-it-shows.png
    logs/                   # captured service logs and CLI output
    tests/                  # test reports
  phase2-cloud/
    screenshots/  logs/  tests/
    resources.txt           # deployed AWS resources (from IaC outputs / describe calls)
  phase3-destroy/
    verify-destroyed.txt
```

- Capture screenshots with a script (`scripts/capture-proof.sh`, driving Playwright headless), never by hand, so they can be reproduced. The screenshots listed in `SPECS.md` are the minimum.
- There is no AWS console login, so render AWS-side visuals from the CLI. For example, `aws cloudwatch get-metric-widget-image` renders a metrics graph as a PNG. Otherwise save CLI `describe`/`get` output as logs.
- Before saving logs or CLI output, redact 12-digit AWS account IDs (`sed -E 's/[0-9]{12}/<ACCOUNT_ID>/g'`), access keys, tokens, passwords and the deployer's public IP.
- Proof must show the system **working**, with real data flowing through it, and not just "deployed".

## 9. Cost and safety guardrails (AWS)

- Use the smallest viable sizes: RDS `db.t4g.micro` single-AZ, Fargate tasks 0.25–0.5 vCPU / 0.5–1 GB × 1 each, one ALB; the Temporal UI listener is restricted to the deployer's current public IP.
- **No NAT gateways.** Also no multi-AZ databases, provisioned IOPS, reserved capacity or Savings Plans, Route 53 domains, or Marketplace subscriptions.
- Databases: `deletion_protection = false`, skip the final snapshot and keep backup retention minimal. S3 buckets are force-destroyable and ECR repos force-deletable. CloudWatch log groups get 1-day retention and are created by IaC, so they get destroyed with it.
- Public app endpoints are fine for the showcase. Keep databases private: their security groups admit only the app tier. Don't open SSH or database ports to `0.0.0.0/0` and don't use SSH at all. Use SSM if you need a shell.
- Don't send real emails or SMS, and don't call paid third-party APIs. Anything external is mocked unless `SPECS.md` says otherwise.
- `scripts/destroy.sh` must be idempotent and safe to re-run. It first deletes anything that would block the IaC destroy: scale ECS services to 0, empty the S3 bucket and ECR repos.
- `scripts/verify-destroyed.sh` lists anything still tagged `Project=pay-by-bank-checkout` (`aws resourcegroupstaggingapi get-resources`) and runs the specific checks in `SPECS.md` §11. It exits non-zero if anything billable remains. The tagging API can show resources for a few minutes after deletion, so confirm each hit with a direct describe call before failing.

## 10. Git and CI

- Use Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `infra:`, `chore:`). Commit small and often, and push to `origin main` at least at the end of every phase.
- `.github/workflows/ci.yml` runs build, lint and the cloud-free tests on every push and pull request. Keep it green.
- Never force-push `main`. Never commit build output, dependency folders (`node_modules`, `target`, `bin`, `obj`, `.venv`) or state files.

## 11. Project-specific rules

- Protobuf is the single source of truth for the API: `proto/pay/v1/*.proto`. Generated Go and TS code is committed under `gen/`, and CI fails if `buf generate` produces a diff.
- Money is `int64` minor units with an ISO currency. Every mutating RPC takes an `idempotency_key`, and the workflow ID is `payment-<checkout_id>`, so duplicate submissions attach to the same workflow (Temporal workflow-ID reuse policy).
- Workflow code must be deterministic: no time, rand or I/O inside workflows. Activities do all the I/O and are idempotent (keyed by `payment_id` + step).
- Live status: `WatchPayment` (server-streaming) pushes state transitions. The API learns about them by querying the workflow, or through a status table the activities update. Pick one, document it in `docs/design/0001`, and keep it simple.
- The SvelteKit checkout is mobile-first and accessible (it's the shopper-facing surface). The React dashboard is a dense operator UI.
- Mobile (Swift/Kotlin) from the post is out of scope for a web showcase. Mention it in the README as the next step (the Connect clients generate for both).

## 12. Definition of done

- [ ] Every stack item in §2 is used in running code. Cross-check with the coverage matrix in `SPECS.md` §2 and tick each row in `PROGRESS.md`.
- [ ] Phase 1 proof is complete, and all tests are green locally and in CI.
- [ ] Phase 2 proof is complete: cloud screenshots, a cloud test run and the resource list.
- [ ] Phase 3: everything is destroyed and `verify-destroyed.txt` shows nothing billable left.
- [ ] `docs/PORTFOLIO.md` and `README.md` are written, and everything is committed and pushed.
