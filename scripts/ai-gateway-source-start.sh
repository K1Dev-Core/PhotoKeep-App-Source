#!/bin/sh
set -eu

SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE=${PHOTOKEEP_GATEWAY_ENV_FILE:-"$SOURCE_DIR/gateway.env"}
if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE; copy gateway.env.example and set the provider key." >&2
  exit 1
fi
set -a
. "$ENV_FILE"
set +a
cd "$SOURCE_DIR"

exec go run ./services/ai-gateway/cmd/photokeep-ai-gateway
