#!/bin/sh
set -eu
# Line trace names the exact failing step; CI job logs are the destination.
PS4='+smoke:${LINENO}: '
set -x

ARCHIVE=${1:?usage: package-smoke.sh <tarball>}
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
VERSION=$(node "$ROOT/scripts/version.mjs")
case "$ARCHIVE" in *"leanote-v${VERSION}-linux-amd64.tar.gz") ;; *) echo "unexpected tarball name" >&2; exit 1;; esac
TMP=$(mktemp -d "${TMPDIR:-/tmp}/leanote-package-smoke.XXXXXX")
CONFIG_DIR=/etc/leanote
CONFIG_FILE=$CONFIG_DIR/app.conf
PID=
CONFIG_CREATED=false
CONFIG_DIR_CREATED=false
cleanup() {
  status=$?
  set +e
  if [ "$status" -ne 0 ] && [ -s "$TMP/app.log" ]; then
    { set +x; } 2>/dev/null
    echo '--- packaged app log (failure diagnostics) ---' >&2
    tail -n 40 "$TMP/app.log" >&2
  fi
  if [ -n "$PID" ]; then kill "$PID" >/dev/null 2>&1; wait "$PID" >/dev/null 2>&1; fi
  if [ "$CONFIG_CREATED" = true ]; then sudo rm -f "$CONFIG_FILE"; fi
  if [ "$CONFIG_DIR_CREATED" = true ]; then sudo rmdir "$CONFIG_DIR" >/dev/null 2>&1 || status=1; fi
  rm -rf "$TMP"
  exit "$status"
}
trap cleanup EXIT INT TERM
tar -xzf "$ARCHIVE" -C "$TMP"
test -f "$TMP/bin/leanote"
test "$(stat -c '%a' "$TMP/bin/leanote")" = 755
test ! -e "$TMP/conf/app.conf"
test ! -e "$TMP/mongodb_backup"
test ! -e "$TMP/files"
test ! -e "$TMP/public/upload"

# Rebuild twice with the same commit timestamp and compare the complete
# archive bytes. The caller may provide the tag timestamp explicitly when the
# checkout is detached from its local Git metadata.
: "${PACKAGE_SMOKE_SOURCE_DATE_EPOCH:=$(git -C "$ROOT" show -s --format=%ct HEAD)}"
REBUILD_A="$TMP/rebuild-a"
REBUILD_B="$TMP/rebuild-b"
mkdir -p "$REBUILD_A" "$REBUILD_B"
SOURCE_DATE_EPOCH="$PACKAGE_SMOKE_SOURCE_DATE_EPOCH" OUTPUT_DIR="$REBUILD_A" sh "$ROOT/sh/package.sh" >/dev/null
SOURCE_DATE_EPOCH="$PACKAGE_SMOKE_SOURCE_DATE_EPOCH" OUTPUT_DIR="$REBUILD_B" sh "$ROOT/sh/package.sh" >/dev/null
HASH_A=$(sha256sum "$REBUILD_A/leanote-v${VERSION}-linux-amd64.tar.gz" | awk '{print $1}')
HASH_B=$(sha256sum "$REBUILD_B/leanote-v${VERSION}-linux-amd64.tar.gz" | awk '{print $1}')
test "$HASH_A" = "$HASH_B"
HASH_INPUT=$(sha256sum "$ARCHIVE" | awk '{print $1}')
test "$HASH_INPUT" = "$HASH_A"

set +e
"$TMP/bin/leanote" -runMode prod >/dev/null 2>&1
code=$?
set -e
test "$code" = 78
set +e
"$TMP/bin/leanote" -runMode dev -conf /etc/leanote/app.conf >/dev/null 2>&1
code=$?
set -e
test "$code" = 78

: "${PACKAGE_SMOKE_MONGODB_URL:?PACKAGE_SMOKE_MONGODB_URL is required}"
: "${PACKAGE_SMOKE_APP_SECRET:?PACKAGE_SMOKE_APP_SECRET is required}"
if [ -e "$CONFIG_FILE" ]; then echo 'refusing to overwrite an existing production config' >&2; exit 1; fi
if [ ! -d "$CONFIG_DIR" ]; then sudo mkdir -p "$CONFIG_DIR"; CONFIG_DIR_CREATED=true; fi
printf '%s\n' '[prod]' 'db.urlEnv=${MONGODB_URL}' 'db.dbname=leanote' 'app.secret=${LEANOTE_APP_SECRET}' 'http.addr=127.0.0.1' 'http.port=19090' > "$TMP/app.conf"
sudo install -o "$(id -u)" -g "$(id -g)" -m 0440 "$TMP/app.conf" "$CONFIG_FILE"
CONFIG_CREATED=true
MONGODB_URL="$PACKAGE_SMOKE_MONGODB_URL" LEANOTE_APP_SECRET="$PACKAGE_SMOKE_APP_SECRET" \
  "$TMP/bin/leanote" -runMode prod -conf "$CONFIG_FILE" >"$TMP/app.log" 2>&1 &
PID=$!
# Cold CI runners need well over 60s for the packaged binary to load
# templates and messages before the first listen; poll for real readiness.
deadline=$(($(date +%s) + 180))
while :; do
  code=$(curl -sS -D "$TMP/healthz.headers" -o "$TMP/healthz" -w '%{http_code}' http://127.0.0.1:19090/healthz || true)
  if [ "$code" = 200 ] && grep -Fx '{"status":"ready"}' "$TMP/healthz" >/dev/null; then break; fi
  if [ "$code" = 503 ] && grep -Fx '{"status":"not_ready"}' "$TMP/healthz" >/dev/null; then
    # A not_ready response during startup ramp is exactly what readiness
    # polling is for: keep polling until the deadline. EXPECT_READY only
    # decides the verdict once the deadline is reached.
    test "${PACKAGE_SMOKE_EXPECT_READY:-false}" = true
    continue
  fi
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'healthz readiness timeout' >&2; exit 1; }
  sleep 1
done
grep -Fi 'Content-Type: application/json; charset=utf-8' "$TMP/healthz.headers" >/dev/null
printf '1\n' > "$TMP/test-marker"
test -s "$TMP/test-marker"
: "${PACKAGE_SMOKE_PDF_URL:?PACKAGE_SMOKE_PDF_URL is required}"
case "$PACKAGE_SMOKE_PDF_URL" in
  */note/toPdf\?*) ;;
  *) echo 'PACKAGE_SMOKE_PDF_URL must target the real /note/toPdf route' >&2; exit 1 ;;
esac
curl -fsS -D "$TMP/pdf.headers" -o "$TMP/pdf.html" "$PACKAGE_SMOKE_PDF_URL"
grep -Eiq '^Content-Type: text/html(?:;|$)' "$TMP/pdf.headers"
test -s "$TMP/pdf.html"
test -x "$(command -v wkhtmltopdf)"
wkhtmltopdf --quiet "$PACKAGE_SMOKE_PDF_URL" "$TMP/smoke.pdf"
test -s "$TMP/smoke.pdf"
test "$(dd if="$TMP/smoke.pdf" bs=1 count=5 2>/dev/null)" = '%PDF-'
