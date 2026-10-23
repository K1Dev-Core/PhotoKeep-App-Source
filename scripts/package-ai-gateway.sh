#!/bin/sh
set -eu

PROJECT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TARGET=${1:-linux/amd64}
case "$TARGET" in
  linux/amd64|linux/arm64|darwin/arm64|darwin/amd64) ;;
  *) echo "Unsupported target: $TARGET" >&2; exit 2 ;;
esac
TARGET_OS=${TARGET%/*}
TARGET_ARCH=${TARGET#*/}
DEFAULT_OUTPUT_DIR="$PROJECT_DIR/dist/photokeep-ai-gateway-$TARGET_OS-$TARGET_ARCH"
OUTPUT_DIR=${2:-$DEFAULT_OUTPUT_DIR}
if [ "$OUTPUT_DIR" = "$DEFAULT_OUTPUT_DIR" ] && [ -d "$OUTPUT_DIR" ]; then
  python3 - "$OUTPUT_DIR" <<'PY'
from pathlib import Path
import shutil
import sys
shutil.rmtree(Path(sys.argv[1]))
PY
fi
mkdir -p "$OUTPUT_DIR"

cd "$PROJECT_DIR"
CGO_ENABLED=0 GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" go build -trimpath -ldflags='-s -w' -o "$OUTPUT_DIR/photokeep-ai-gateway" ./services/ai-gateway/cmd/photokeep-ai-gateway
cp scripts/ai-gateway-start.sh "$OUTPUT_DIR/start.sh"
cp scripts/ai-gateway.env.example "$OUTPUT_DIR/gateway.env.example"
cp docs/remote-ai-gateway.md "$OUTPUT_DIR/README.md"
chmod 755 "$OUTPUT_DIR/photokeep-ai-gateway" "$OUTPUT_DIR/start.sh"

(
  cd "$OUTPUT_DIR"
  shasum -a 256 photokeep-ai-gateway > SHA256SUMS
)
tar -czf "$OUTPUT_DIR.tar.gz" -C "$(dirname "$OUTPUT_DIR")" "$(basename "$OUTPUT_DIR")"
echo "Gateway package: $OUTPUT_DIR"
echo "Transfer archive: $OUTPUT_DIR.tar.gz"
