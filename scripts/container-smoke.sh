#!/bin/sh
set -eu

IMAGE=${1:?usage: container-smoke.sh <image>}
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
MONGO=leanote-container-smoke-mongo
APP=leanote-container-smoke-app
GOTENBERG=leanote-container-smoke-gotenberg
NETWORK=leanote-container-smoke
PDF_NETWORK=leanote-container-smoke-pdf
# Same pinned image and hardening flags as docker-compose.yml.
GOTENBERG_IMAGE=docker.io/gotenberg/gotenberg:8.37.0@sha256:f29984bd1e226bf1b93ba90af06000afa8b315853e99d27b9aaa41b93f15c769
TMP_HEALTH=$(mktemp)
TMP_CONFIG=$(mktemp)
# Host directories for the documented persistent volumes
# (docs/modernization/cicd-delivery.md): /var/lib/leanote/{private,public,backup}.
PRIVATE_DIR=$(mktemp -d)
PUBLIC_DIR=$(mktemp -d)
BACKUP_DIR=$(mktemp -d)
FILES_DIR="$PRIVATE_DIR/files"
UPLOAD_DIR="$PUBLIC_DIR/upload"
mkdir -p "$FILES_DIR" "$PRIVATE_DIR/quarantine" "$UPLOAD_DIR" "$PUBLIC_DIR/quarantine"
chmod 0777 "$PRIVATE_DIR" "$PUBLIC_DIR" "$BACKUP_DIR" "$PRIVATE_DIR/quarantine" "$PUBLIC_DIR/quarantine"
chmod 0777 "$FILES_DIR" "$UPLOAD_DIR"
cleanup() {
  status=$?
  set +e
  cleanup_error=0
  if docker inspect "$APP" >/dev/null 2>&1; then docker rm -f "$APP" >/dev/null || cleanup_error=1; fi
  if docker inspect "$GOTENBERG" >/dev/null 2>&1; then docker rm -f "$GOTENBERG" >/dev/null || cleanup_error=1; fi
  if docker inspect "$MONGO" >/dev/null 2>&1; then docker rm -f "$MONGO" >/dev/null || cleanup_error=1; fi
  if docker network inspect "$PDF_NETWORK" >/dev/null 2>&1; then docker network rm "$PDF_NETWORK" >/dev/null || cleanup_error=1; fi
  if docker network inspect "$NETWORK" >/dev/null 2>&1; then docker network rm "$NETWORK" >/dev/null || cleanup_error=1; fi
  rm -f "$TMP_HEALTH" "$TMP_HEALTH.headers" "$TMP_HEALTH.pdf.headers" "$TMP_HEALTH.pdf.html" "$TMP_HEALTH.login" "$TMP_HEALTH.export.headers" "$TMP_HEALTH.export.pdf" "$TMP_CONFIG"
  rm -rf "$PRIVATE_DIR" "$PUBLIC_DIR" "$BACKUP_DIR"
  test ! -e "$TMP_HEALTH" && test ! -e "$TMP_CONFIG" || cleanup_error=1
  if [ "$status" -eq 0 ] && [ "$cleanup_error" -ne 0 ]; then status=1; fi
  exit "$status"
}
trap cleanup EXIT INT TERM
docker rm -f "$APP" "$GOTENBERG" "$MONGO" >/dev/null 2>&1 || true
docker network create "$NETWORK" >/dev/null
# The PDF network is internal: Gotenberg has no external egress and no host port.
docker network create --internal "$PDF_NETWORK" >/dev/null
docker run -d --name "$GOTENBERG" --network "$PDF_NETWORK" --network-alias gotenberg   --health-cmd 'curl -fsS http://127.0.0.1:3000/health'   --health-interval 2s --health-timeout 2s --health-retries 30   "$GOTENBERG_IMAGE" gotenberg --api-timeout=40s --libreoffice-disable-routes=true   --pdfengines-disable-routes=true --webhook-disable=true   '--chromium-deny-list=^(?!file:///tmp/|data:).*' >/dev/null
docker run -d --name "$MONGO" --network "$NETWORK" \
  --health-cmd 'mongosh --quiet --eval "db.runCommand({ping:1}).ok"' \
  --health-interval 2s --health-timeout 2s --health-retries 30 \
  docker.io/library/mongo:8.0@sha256:376f5173003b5408d7b8e6989667231c0bf0cefdce379d7c814910429d1a7a85 >/dev/null
deadline=$(($(date +%s) + 90))
while [ "$(docker inspect -f '{{.State.Health.Status}}' "$MONGO")" != healthy ]; do
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'MongoDB readiness timeout' >&2; exit 1; }
  sleep 1
done
while [ "$(docker inspect -f '{{.State.Health.Status}}' "$GOTENBERG")" != healthy ]; do
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'Gotenberg readiness timeout; logs:' >&2; docker logs --tail 40 "$GOTENBERG" >&2 || true; exit 1; }
  sleep 1
done
docker cp "$ROOT/mongodb_backup/leanote_install_data" "$MONGO:/leanote_install_data"
docker exec "$MONGO" mongorestore --db leanote --dir /leanote_install_data --drop >/dev/null
printf '%s\n' '[prod]' 'db.urlEnv=${MONGODB_URL}' 'db.dbname=leanote' 'app.secret=${LEANOTE_APP_SECRET}' 'http.addr=0.0.0.0' 'http.port=9000' \
  'content.private.data=/var/lib/leanote/private/files' \
  'content.private.quarantine=/var/lib/leanote/private/quarantine' \
  'content.public.data=/var/lib/leanote/public/upload' \
  'content.public.quarantine=/var/lib/leanote/public/quarantine' \
  'content.temporary=/var/lib/leanote/tmp' \
  'admin.backup.root=/var/lib/leanote/backup' \n  'pdf.renderer=gotenberg' 'pdf.gotenberg.url=http://gotenberg:3000' > "$TMP_CONFIG"
chmod 0440 "$TMP_CONFIG"
docker run -d --name "$APP" --user 10001:10001 --group-add "$(id -g)" --network "$NETWORK" -p 9000:9000 \
  -v "$TMP_CONFIG:/etc/leanote/app.conf:ro" \
  -v "$PRIVATE_DIR:/var/lib/leanote/private" -v "$PUBLIC_DIR:/var/lib/leanote/public" -v "$BACKUP_DIR:/var/lib/leanote/backup" \
  -e MONGODB_URL="mongodb://$MONGO:27017/leanote" \
  -e LEANOTE_APP_SECRET='container-smoke-secret-012345678901234567890' "$IMAGE" >/dev/null
docker network connect "$PDF_NETWORK" "$APP"
deadline=$(($(date +%s) + 180))
while :; do
  code=$(curl -sS -D "$TMP_HEALTH.headers" -o "$TMP_HEALTH" -w '%{http_code}' http://127.0.0.1:9000/healthz || true)
  if [ "$code" = 200 ] && grep -Fx '{"status":"ready"}' "$TMP_HEALTH" >/dev/null; then break; fi
  if [ "$code" = 503 ] && grep -Fx '{"status":"not_ready"}' "$TMP_HEALTH" >/dev/null; then
    # A not_ready response during container startup is transient by design;
    # the deadline below is the only failure point for readiness.
    sleep 1
    continue
  fi
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'healthz readiness timeout; app logs:' >&2; docker logs --tail 40 "$APP" >&2 || true; exit 1; }
  sleep 1
done
grep -Fi 'Content-Type: application/json; charset=utf-8' "$TMP_HEALTH.headers" >/dev/null
: "${CONTAINER_SMOKE_PDF_URL:?CONTAINER_SMOKE_PDF_URL is required}"
case "$CONTAINER_SMOKE_PDF_URL" in
  */note/toPdf\?*) ;;
  *) echo 'CONTAINER_SMOKE_PDF_URL must target the legacy /note/toPdf route' >&2; exit 1 ;;
esac
# The legacy appKey callback was retired (158a6de): it must stay a stub and
# never render or leak note content, even with the correct secret as appKey.
curl -fsS -D "$TMP_HEALTH.pdf.headers" -o "$TMP_HEALTH.pdf.html" "$CONTAINER_SMOKE_PDF_URL"
grep -Eiq '^Content-Type: text/plain(;|$)' "$TMP_HEALTH.pdf.headers"
test "$(cat "$TMP_HEALTH.pdf.html")" = 'no note'
if grep -Eiq 'About Leanote|not just a notepad' "$TMP_HEALTH.pdf.html"; then echo 'legacy /note/toPdf leaked note content' >&2; exit 1; fi
# Persistent private volume is writable by the non-root user.
docker exec "$APP" sh -c 'printf persisted > /var/lib/leanote/private/files/smoke-marker'
# Real export through the application: the app must reach Gotenberg over the
# internal PDF network and the output must pass the Leanote PDF validation.
# The fixture user and note come from mongodb_backup/leanote_install_data; the
# note's original remote image host is gone, so give it self-contained CJK content.
docker exec "$MONGO" mongosh --quiet leanote --eval 'db.note_contents.updateOne({_id: ObjectId("540817e099c37b583c000005")}, {$set: {Content: "<h1>中文标题 Container PDF smoke</h1><p>这是一段中文正文。</p><table border=\"1\"><tr><td>表格</td><td>单元格</td></tr></table>"}}).modifiedCount' | grep -qx 1
curl -fsS -G -o "$TMP_HEALTH.login" --data-urlencode 'email=demo@leanote.com' --data-urlencode 'pwd=demo@leanote.com' http://127.0.0.1:9000/api/auth/login
token=$(sed -n 's/.*"Token":"\([^"]*\)".*/\1/p' "$TMP_HEALTH.login")
test -n "$token" || { echo 'fixture login did not return a token' >&2; exit 1; }
curl -fsS -G -D "$TMP_HEALTH.export.headers" -o "$TMP_HEALTH.export.pdf" \
  --data-urlencode "token=$token" --data-urlencode 'noteId=540817e099c37b583c000005' http://127.0.0.1:9000/api/note/exportPdf \
  || { echo 'PDF export failed; logs:' >&2; docker logs --tail 40 "$GOTENBERG" >&2 || true; docker logs --tail 40 "$APP" >&2 || true; exit 1; }
grep -Eiq '^Content-Type: application/pdf' "$TMP_HEALTH.export.headers"
test "$(dd if="$TMP_HEALTH.export.pdf" bs=1 count=5 2>/dev/null)" = '%PDF-'
# Gotenberg must have no external egress (internal network, no host port).
if docker exec "$GOTENBERG" curl -fsS --max-time 5 https://example.com >/dev/null 2>&1; then echo 'gotenberg unexpectedly reached the internet' >&2; exit 1; fi
printf persisted-upload > "$UPLOAD_DIR/smoke-upload"
docker restart "$APP" >/dev/null
test -f "$FILES_DIR/smoke-marker"
test "$(docker exec "$APP" cat /var/lib/leanote/private/files/smoke-marker)" = persisted
test "$(docker exec "$APP" cat /var/lib/leanote/public/upload/smoke-upload)" = persisted-upload
deadline=$(($(date +%s) + 180))
while :; do
  code=$(curl -sS -D "$TMP_HEALTH.headers" -o "$TMP_HEALTH" -w '%{http_code}' http://127.0.0.1:9000/healthz || true)
  if [ "$code" = 200 ] && grep -Fx '{"status":"ready"}' "$TMP_HEALTH" >/dev/null; then break; fi
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'healthz readiness timeout after restart' >&2; exit 1; }
  sleep 1
done
