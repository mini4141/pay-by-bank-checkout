#!/usr/bin/env bash
# Usage: scripts/capture-proof.sh phase1|phase2
# Re-runs the tests/e2e story with screenshots and collects logs into proof/<phase>/.
set -euo pipefail
cd "$(dirname "$0")/.."
PHASE="${1:?usage: capture-proof.sh phase1|phase2}"

redact() { sed -E 's/[0-9]{12}/<ACCOUNT_ID>/g; s/(AKIA|ASIA)[A-Z0-9]{16}/<ACCESS_KEY>/g'; }

case "$PHASE" in
  phase1)
    P=proof/phase1-local
    rm -rf "$P"; mkdir -p "$P/screenshots" "$P/logs" "$P/tests"
    export PROOF_DIR="$(pwd)/$P" TEST_OUT="$P/tests"
    ./scripts/test-all.sh 2>&1 | redact | tee "$P/logs/test-all.log"
    cp "$P/tests/buf-lint.txt" "$P/logs/buf-lint.txt"; [ -s "$P/logs/buf-lint.txt" ] || echo "buf lint: no findings" > "$P/logs/buf-lint.txt"
    docker compose logs --no-color worker 2>&1 | redact > "$P/logs/worker.log"
    docker compose logs --no-color mockbank 2>&1 | grep -E 'injected_failure|replayed' | redact > "$P/logs/mockbank-failures.log" || true
    docker compose logs --no-color api receiver 2>&1 | redact > "$P/logs/api-receiver.log"
    ;;
  phase2)
    ./scripts/capture-cloud-proof.sh
    ;;
  *) echo "unknown phase $PHASE" >&2; exit 2 ;;
esac
echo "proof captured in proof/$PHASE*"
