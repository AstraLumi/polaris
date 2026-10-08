#!/usr/bin/env bash
# Pull the latest code and rebuild the container. Run it from anywhere:
#   ./scripts/update.sh
# Your .env and your data volumes are never touched.
set -euo pipefail
cd "$(dirname "$0")/.."
git pull --ff-only
docker compose up -d --build
docker image prune -f >/dev/null
docker compose ps
