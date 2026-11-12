#!/bin/sh
# Browser tests use a fresh database and never touch the user's documentation.
set -eu
cd "$(dirname "$0")/.."
test_directory=$(mktemp -d "${TMPDIR:-/tmp}/yanetviz-e2e.XXXXXX")
server_pid=
cleanup() {
  if [ -n "$server_pid" ]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$test_directory"
}
trap cleanup EXIT
trap 'exit 0' INT TERM
go build -buildvcs=false -o "$test_directory/yanetviz" ./cmd/yanetviz
YANETVIZ_PASSWORD="${YANETVIZ_TEST_PASSWORD:-}" "$test_directory/yanetviz" \
  -listen "127.0.0.1:${YANETVIZ_TEST_PORT:-18080}" -data "$test_directory/data" \
  -public-view="${YANETVIZ_TEST_PUBLIC_VIEW:-false}" &
server_pid=$!
wait "$server_pid"
