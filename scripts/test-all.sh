#!/usr/bin/env bash
# Runs every local test layer and exits non-zero if any fails.
# TEST_OUT: where reports go (default .local/test-results).
# PROOF_DIR: when set, the Playwright story also saves its screenshots there.
# SKIP_E2E=1: skip the stack-dependent layers (used by CI jobs that have no compose stack).
set -uo pipefail
cd "$(dirname "$0")/.."
OUT="$(mkdir -p "${TEST_OUT:-.local/test-results}" && cd "${TEST_OUT:-.local/test-results}" && pwd)"
export DATABASE_URL="${DATABASE_URL:-postgres://pay:pay@localhost:5433/pay?sslmode=disable}"
FAILED=()
run() { local name="$1"; shift; echo "=== $name"; if "$@"; then echo "--- $name: ok"; else echo "--- $name: FAILED"; FAILED+=("$name"); fi; }

if [ -z "${SKIP_E2E:-}" ]; then
  ./scripts/dev-up.sh >/dev/null || FAILED+=("dev-up")
fi

run "buf lint" bash -c "buf lint | tee '$OUT/buf-lint.txt'; test \${PIPESTATUS[0]} -eq 0"
run "buf generate is clean" bash -c "buf generate && git diff --exit-code -- gen"
run "go vet" go vet ./...
command -v go-junit-report >/dev/null || go install github.com/jstemmer/go-junit-report/v2@latest
run "go test (unit + sqlc integration)" bash -c "go test -v -count=1 ./... 2>&1 | tee '$OUT/go-test.log' | go-junit-report -set-exit-code > '$OUT/go-junit.xml'"
run "svelte-check" pnpm --filter @pay/checkout check
run "vitest (svelte)" pnpm --filter @pay/checkout test
run "tsc (react)" pnpm --filter @pay/dashboard check
run "vitest (react)" pnpm --filter @pay/dashboard test
cp -f .local/test-results/vitest-*.xml "$OUT/" 2>/dev/null || true

if [ -z "${SKIP_E2E:-}" ]; then
  run "playwright e2e" bash -c "cd e2e && PW_REPORT_DIR='$OUT/playwright' pnpm exec playwright test"
fi

if [ ${#FAILED[@]} -gt 0 ]; then echo "FAILED: ${FAILED[*]}"; exit 1; fi
echo "ALL GREEN"
