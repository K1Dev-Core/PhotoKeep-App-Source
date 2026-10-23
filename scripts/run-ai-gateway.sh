#!/bin/sh
set -eu
PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$PROJECT_DIR"
ENV_FILE=${PHOTOKEEP_GATEWAY_ENV_FILE:-"$PROJECT_DIR/.env.gateway"}
if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE; copy scripts/ai-gateway.env.example and configure a provider." >&2
  exit 1
fi
set -a
. "$ENV_FILE"
set +a
exec go run ./services/ai-gateway/cmd/photokeep-ai-gateway
