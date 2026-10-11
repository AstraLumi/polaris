#!/usr/bin/env bash
# Update Polaris and restart it. Run it from anywhere:
#   ./scripts/update.sh           the latest release's ready-made image
#   ./scripts/update.sh --build   build from the latest code instead
# Your .env and your data volumes are never touched.
set -euo pipefail
cd "$(dirname "$0")/.."
git pull --ff-only
if [ "${1:-}" = "--build" ]; then
  docker compose up -d --build
else
  docker compose pull
  docker compose up -d
fi
docker image prune -f >/dev/null
docker compose ps
