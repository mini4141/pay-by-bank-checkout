# ADR 0002: CloudFront also fronts the API for the dashboard

Status: accepted · Date: 2026-11-26

## Context

SPECS §10 says the dashboard SPA (S3 + CloudFront, HTTPS) calls the ALB DNS name directly. The ALB has no certificate (no domain; Route 53 is out of scope), so it only speaks HTTP. A page served over HTTPS cannot call an HTTP API: browsers block it as mixed content.

## Decision

Add a second CloudFront origin (the ALB, HTTP-only) with a `/api/*` behaviour (caching disabled, all headers and methods forwarded). The dashboard is built with `VITE_API_URL=/api`, so it is same-origin and needs no CORS. The shopper checkout still talks to the ALB directly over HTTP (same origin as the ALB, so no CORS either).

## Consequences

- Server-streaming RPCs pass through CloudFront. Streams are kept short (the server ends them after 20 s and clients re-subscribe), which keeps them under CloudFront's origin read timeout.
- No custom domain or certificate is needed; the whole stack stays destroyable with Terraform.

## Update: account not verified for CloudFront

During the first deploy AWS rejected `CreateDistribution` with `403 AccessDenied: Your account must be verified before you can add new CloudFront resources`. Only AWS Support can lift that. The distribution therefore sits behind the Terraform variable `enable_cloudfront` (default `false`). When it is off, the dashboard is served by S3 static website hosting (public-read bucket, plain HTTP) and calls the ALB API directly; the API's CORS policy allows the S3 website origin. Because both are HTTP, there is no mixed-content problem and the CloudFront `/api/*` proxy is not needed.
