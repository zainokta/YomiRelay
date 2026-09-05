#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

for command_name in go node npm; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'Error: %s is required.\n' "$command_name" >&2
    exit 1
  fi
done

NODE_RAW="$(node --version)"
NODE_VERSION="${NODE_RAW#v}"
if [[ ! "$NODE_VERSION" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
  printf 'Error: Node.js >= 22.22.2 is required.\nInstalled: %s\n' "$NODE_VERSION" >&2
  exit 1
fi
NODE_MAJOR="${BASH_REMATCH[1]}" NODE_MINOR="${BASH_REMATCH[2]}" NODE_PATCH="${BASH_REMATCH[3]}"
if (( NODE_MAJOR < 22 || (NODE_MAJOR == 22 && NODE_MINOR < 22) || (NODE_MAJOR == 22 && NODE_MINOR == 22 && NODE_PATCH < 2) )); then
  printf 'Error: Node.js >= 22.22.2 is required.\nInstalled: %s\n' "$NODE_VERSION" >&2
  exit 1
fi

cd "$ROOT_DIR/frontend"
if [[ ! -d node_modules ]]; then
  npm ci
fi

BACKEND_PID=""
FRONTEND_PID=""
CLEANED=0
cleanup() {
  if (( CLEANED )); then return; fi
  CLEANED=1
  for pid in "$BACKEND_PID" "$FRONTEND_PID"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill -TERM "$pid" 2>/dev/null || true
    fi
  done
  for pid in "$BACKEND_PID" "$FRONTEND_PID"; do
    if [[ -n "$pid" ]]; then
      wait "$pid" 2>/dev/null || true
    fi
  done
}
trap cleanup EXIT INT TERM

printf '[backend] starting YomiRelay\n'
( cd "$ROOT_DIR" && exec go run . ) &
BACKEND_PID=$!
printf '[frontend] starting Vite server\n'
( cd "$ROOT_DIR/frontend" && exec npm run dev ) &
FRONTEND_PID=$!
printf 'YomiRelay development processes started.\n'
printf 'Development frontend: http://127.0.0.1:5173\n'
set +e
wait -n "$BACKEND_PID" "$FRONTEND_PID"
STATUS=$?
set -e
cleanup
exit "$STATUS"
