#!/bin/sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$PROJECT_DIR"

if [ -z "${PHOTOKEEP_AI_GATEWAY_URL+x}" ] && [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  . "$PROJECT_DIR/.env"
  set +a
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required to run PhotoKeep. Install Go, then run this script again." >&2
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "Node.js and npm are required to run PhotoKeep. Install them, then run this script again." >&2
  exit 1
fi

if [ ! -d frontend/node_modules ]; then
  echo "Installing frontend dependencies..."
  (cd frontend && npm install --legacy-peer-deps)
fi

GATEWAY_PID=
GATEWAY_TMP=
cleanup() {
  if [ -n "$GATEWAY_PID" ]; then
    kill "$GATEWAY_PID" 2>/dev/null || true
  fi
  if [ -n "$GATEWAY_TMP" ]; then
    rm -rf "$GATEWAY_TMP"
  fi
}
trap cleanup EXIT

case "${PHOTOKEEP_AI_GATEWAY_URL:-http://127.0.0.1:8081/v1/analyze}" in
  http://127.0.0.1:8081/*|http://localhost:8081/*)
    START_LOCAL_GATEWAY=1
    ;;
  *)
    START_LOCAL_GATEWAY=0
    ;;
esac
if [ "$START_LOCAL_GATEWAY" -eq 1 ] && ! curl -sS --max-time 3 -o /dev/null http://127.0.0.1:8081/health >/dev/null 2>&1; then
  GATEWAY_ENV=${PHOTOKEEP_GATEWAY_ENV_FILE:-"$PROJECT_DIR/.env.gateway"}
  if [ ! -f "$GATEWAY_ENV" ]; then
    echo "Copy scripts/ai-gateway.env.example to .env.gateway and configure the provider." >&2
    exit 1
  fi
  GATEWAY_TMP=$(mktemp -d)
  go build -o "$GATEWAY_TMP/photokeep-ai-gateway" ./services/ai-gateway/cmd/photokeep-ai-gateway
  (
    set -a
    . "$GATEWAY_ENV"
    set +a
    exec "$GATEWAY_TMP/photokeep-ai-gateway"
  ) >"$GATEWAY_TMP/gateway.log" 2>&1 &
  GATEWAY_PID=$!
  READY=0
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    if curl -sS --max-time 3 -o /dev/null http://127.0.0.1:8081/health >/dev/null 2>&1; then
      READY=1
      break
    fi
    if ! kill -0 "$GATEWAY_PID" 2>/dev/null; then
      break
    fi
    sleep 1
  done
  if [ "$READY" -ne 1 ]; then
    cat "$GATEWAY_TMP/gateway.log" >&2
    echo "PhotoKeep AI gateway did not become ready." >&2
    exit 1
  fi
fi

go run github.com/wailsapp/wails/v2/cmd/wails@v2.16.0 dev
