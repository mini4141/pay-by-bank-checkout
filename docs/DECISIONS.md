# Decisions

One line each: date, decision, reason.

- 2026-11-26 Go toolchain on host is 1.27.4; `go.mod` declares `go 1.25` (the project floor); newer toolchain is backward compatible.
- 2026-11-26 Single Go module at repo root (`github.com/mini4141/pay-by-bank-checkout`) shared by api, worker, mockbank, receiver; simplest way to share generated code and sqlc package.
- 2026-11-26 Node on host is 24.12, pnpm 12; sqlc 1.30, goose 3.26 installed via `go install`; buf 1.75 already present.
- 2026-11-26 Temporal dev server runs from the official `temporalio/temporal` image (no host CLI install needed), locally and on Fargate.
- 2026-11-26 Status propagation uses a status table (`payment_events`) written by activities and polled by the streaming RPCs; see ADR 0001.
- 2026-11-26 Dependencies (Temporal SDK / pgx) pull `go.mod` to `go 1.26.0`; Docker builder image is `golang:1.26-alpine`. Reason: library minimum, newer than the Go 1.25 floor in the rules.
- 2026-11-26 CloudFront could not be created: AWS returned 403 "Your account must be verified before you can add new CloudFront resources" (needs AWS Support). Dashboard is served from S3 static website hosting instead (public-read bucket, API CORS allows its origin); CloudFront stays in Terraform behind `enable_cloudfront` (default false). See ADR 0002.
