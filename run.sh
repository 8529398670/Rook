#!/usr/bin/env bash
# Build this machine's binary and run it. The same command every time:
#
#   ./run.sh
#
# It always builds first, so what runs is the code as it is now, and the
# binary's name never changes (dist/rook_<os>_<arch>). A copy of the server
# still running from an earlier ./run.sh is stopped first -- politely, so it
# finishes saving -- since it would otherwise hold the port, or keep serving
# the old code.
#
# Anything after ./run.sh goes to the binary: ./run.sh manage list-users
set -euo pipefail
cd "$( dirname "${BASH_SOURCE[0]}" )"

BINARY="dist/rook_$( go env GOOS )_$( go env GOARCH )"

QUIET=1 ./build.sh "$( go env GOOS )/$( go env GOARCH )"

# Only the server needs the old copy gone; "manage" talks to it instead.
if [ "${1:-}" != "manage" ] && [ "${1:-}" != "version" ]; then
  OLD="$( pgrep -f "$( pwd )/dist/rook_|\./dist/rook_|^dist/rook_" || true )"
  if [ -n "$OLD" ]; then
    echo "Stopping the server that was already running (pid ${OLD//$'\n'/ })"
    kill $OLD 2>/dev/null || true
    for _ in $( seq 1 50 ); do
      pgrep -f "$( pwd )/dist/rook_|\./dist/rook_|^dist/rook_" >/dev/null || break
      sleep 0.1
    done
  fi
fi

echo ""
exec "./$BINARY" "$@"
