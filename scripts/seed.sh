#!/usr/bin/env bash
# Creates the demo merchant "Kissa Coffee" and stores its API key in .local/seed.json.
# Idempotent: reuses the stored key while it is still valid.
set -euo pipefail
cd "$(dirname "$0")/.."

API_URL="${API_URL:-http://localhost:8080/api}"
SEED_FILE="${SEED_FILE:-.local/seed.json}"
WEBHOOK_URL="${DEMO_WEBHOOK_URL:-http://receiver:8082/receiver/webhook}"
WEBHOOK_SECRET="${DEMO_WEBHOOK_SECRET:-whsec_demo_local_only}"
mkdir -p "$(dirname "$SEED_FILE")"

rpc() { curl -fsS -X POST -H 'Content-Type: application/json' "${@:2}" "$API_URL/$1"; }

if [ -f "$SEED_FILE" ]; then
  KEY=$(sed -n 's/.*"api_key": *"\([^"]*\)".*/\1/p' "$SEED_FILE")
  if rpc pay.v1.MerchantService/GetBalance -H "Authorization: Bearer $KEY" -d '{}' >/dev/null 2>&1; then
    echo "seed: reusing merchant from $SEED_FILE"
    exit 0
  fi
fi

RES=$(rpc pay.v1.MerchantService/CreateMerchant -d "{\"name\":\"Kissa Coffee\",\"webhookUrl\":\"$WEBHOOK_URL\",\"webhookSecret\":\"$WEBHOOK_SECRET\",\"idempotencyKey\":\"seed-$(date +%s)-$RANDOM\"}")
KEY=$(echo "$RES" | sed -n 's/.*"apiKey": *"\([^"]*\)".*/\1/p')
MID=$(echo "$RES" | sed -n 's/.*"merchantId": *"\([^"]*\)".*/\1/p')
[ -n "$KEY" ] || { echo "seed: CreateMerchant returned no API key: $RES" >&2; exit 1; }
printf '{"merchant_id": "%s", "api_key": "%s"}\n' "$MID" "$KEY" > "$SEED_FILE"
echo "seed: created merchant $MID (key stored in $SEED_FILE)"
