#!/bin/sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
OUTPUT_DIR=${1:-"$PROJECT_DIR/dist/photokeep-ai-gateway-source"}

if [ "$OUTPUT_DIR" = "$PROJECT_DIR/dist/photokeep-ai-gateway-source" ]; then
  if [ -d "$OUTPUT_DIR" ]; then
    python3 - "$OUTPUT_DIR" <<'PY'
from pathlib import Path
import shutil
import sys
shutil.rmtree(Path(sys.argv[1]))
PY
  fi
elif [ -e "$OUTPUT_DIR" ]; then
  echo "Output already exists: $OUTPUT_DIR" >&2
  echo "Choose a new output path or use the default generated package path." >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR/services/ai-gateway/cmd/photokeep-ai-gateway" \
  "$OUTPUT_DIR/services/ai-gateway/internal/gateway" "$OUTPUT_DIR/internal/photokeep"

cp "$PROJECT_DIR/go.mod" "$PROJECT_DIR/go.sum" "$OUTPUT_DIR/"
cp "$PROJECT_DIR/services/ai-gateway/cmd/photokeep-ai-gateway/main.go" "$OUTPUT_DIR/services/ai-gateway/cmd/photokeep-ai-gateway/"
for source in "$PROJECT_DIR/services/ai-gateway/internal/gateway/"*.go; do
  case "$source" in *_test.go) continue ;; esac
  cp "$source" "$OUTPUT_DIR/services/ai-gateway/internal/gateway/"
done
for source in "$PROJECT_DIR/internal/photokeep/"*.go; do
  case "$source" in *_test.go) continue ;; esac
  cp "$source" "$OUTPUT_DIR/internal/photokeep/"
done
cp "$PROJECT_DIR/scripts/ai-gateway-source-start.sh" "$OUTPUT_DIR/start.sh"
cp "$PROJECT_DIR/scripts/ai-gateway.env.example" "$OUTPUT_DIR/gateway.env.example"
cp "$PROJECT_DIR/docs/remote-ai-gateway-source.md" "$OUTPUT_DIR/README.md"
chmod 755 "$OUTPUT_DIR/start.sh"
(cd "$OUTPUT_DIR" && go mod tidy)
tar -czf "$OUTPUT_DIR.tar.gz" -C "$(dirname "$OUTPUT_DIR")" "$(basename "$OUTPUT_DIR")"
echo "Gateway source: $OUTPUT_DIR"
echo "Transfer archive: $OUTPUT_DIR.tar.gz"
