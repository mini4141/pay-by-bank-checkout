#!/usr/bin/env bash
# Starts the whole stack locally with Docker Compose and seeds the demo merchant.
set -euo pipefail
cd "$(dirname "$0")/.."

docker compose up -d --build
echo "waiting for the API..."
for _ in $(seq 1 60); do
  curl -fsS http://localhost:8080/healthz >/dev/null 2>&1 && break
  sleep 2
done
curl -fsS http://localhost:8080/healthz >/dev/null
./scripts/seed.sh
cat <<MSG

Stack is up:
  Merchant dashboard  http://localhost:5173   (API key: .local/seed.json)
  Shopper checkout    http://localhost:3000
  API (Connect)       http://localhost:8080/api
  Temporal UI         http://localhost:8233
  Mock bank           http://localhost:8081   (fail rate: POST /admin/fail-rate)
  Webhook receiver    http://localhost:8082/receiver/deliveries
MSG
