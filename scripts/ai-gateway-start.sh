#!/bin/sh
set -eu

PACKAGE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ENV_FILE=${PHOTOKEEP_GATEWAY_ENV_FILE:-"$PACKAGE_DIR/gateway.env"}
if [ ! -f "$ENV_FILE" ]; then
  echo "Missing $ENV_FILE; copy gateway.env.example and set the provider key on this server." >&2
  exit 1
fi
set -a
. "$ENV_FILE"
set +a

exec "$PACKAGE_DIR/photokeep-ai-gateway"
