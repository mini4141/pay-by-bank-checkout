#!/usr/bin/env bash
# Stops the local stack and removes its volumes.
set -euo pipefail
cd "$(dirname "$0")/.."
docker compose down -v --remove-orphans
rm -f .local/seed.json
