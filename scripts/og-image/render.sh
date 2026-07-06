#!/usr/bin/env bash
# Render og-image.html to the site's OG image (2400x1260 PNG = 1200x630 @2x)
# using headless Chrome.
#
# Usage:
#   ./scripts/og-image/render.sh                 # writes docs/public/og-image.png
#   ./scripts/og-image/render.sh /path/out.png   # writes a custom path (e.g. a draft)
#
# Override the browser binary with CHROME=/path/to/chrome if autodetection fails.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$DIR/../.." && pwd)"
OUT="${1:-$REPO_ROOT/docs/public/og-image.png}"

find_chrome() {
  if [[ -n "${CHROME:-}" ]]; then echo "$CHROME"; return; fi
  local candidates=(
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    "/Applications/Chromium.app/Contents/MacOS/Chromium"
    "$(command -v google-chrome || true)"
    "$(command -v chromium || true)"
    "$(command -v chromium-browser || true)"
  )
  for c in "${candidates[@]}"; do
    [[ -n "$c" && -x "$c" ]] && { echo "$c"; return; }
  done
  echo "error: no Chrome/Chromium binary found; set CHROME=/path/to/chrome" >&2
  exit 1
}

CHROME_BIN="$(find_chrome)"

"$CHROME_BIN" \
  --headless=new \
  --disable-gpu \
  --hide-scrollbars \
  --force-device-scale-factor=2 \
  --window-size=1200,630 \
  --virtual-time-budget=3000 \
  --screenshot="$OUT" \
  "file://$DIR/og-image.html"

echo "Rendered: $OUT"
