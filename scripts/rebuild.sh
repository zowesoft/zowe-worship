#!/usr/bin/env bash
# Rebuild the packaged app and relaunch it: the one command the agent notes ask
# for. Exits non-zero when the build fails, the binary was not replaced, or the
# app did not come up with its window.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

wails3_bin="${WAILS3:-/c/Users/JohnCode/go/bin/wails3}"
if [ ! -x "$wails3_bin" ]; then
  wails3_bin="$(command -v wails3 || true)"
fi
if [ -z "$wails3_bin" ]; then
  echo "rebuild: wails3 not found — set WAILS3 to its path" >&2
  exit 1
fi

exe="bin/zowe-worship.exe"
log="${TMPDIR:-/tmp}/zowe-rebuild.log"
before=$(stat -c %Y "$exe" 2>/dev/null || echo 0)

echo "rebuild: building (log: $log)"
if ! "$wails3_bin" build >"$log" 2>&1; then
  echo "rebuild: build failed" >&2
  tail -n 30 "$log" >&2
  exit 1
fi

after=$(stat -c %Y "$exe" 2>/dev/null || echo 0)
if [ "$after" = "$before" ]; then
  echo "rebuild: build reported success but $exe was not replaced" >&2
  tail -n 30 "$log" >&2
  exit 1
fi

# Build side effects the agent notes call out.
rm -rf frontend/frontend
git checkout -- frontend/dist/.gitkeep 2>/dev/null || true

# wails3 build stops running instances itself, so this always starts the new one.
cmd //c start "" "bin\\zowe-worship.exe"
sleep 5

pid=$(powershell -NoProfile -Command \
  "Get-Process -Name zowe-worship -ErrorAction SilentlyContinue | Where-Object { \$_.MainWindowTitle -eq 'Zowe Worship' } | Select-Object -First 1 -ExpandProperty Id" \
  | tr -d '\r' | head -n 1)
if [ -z "$pid" ]; then
  echo "rebuild: the app did not come up with the Zowe Worship window" >&2
  exit 1
fi

echo "rebuild: $exe rebuilt and running (pid $pid)"
