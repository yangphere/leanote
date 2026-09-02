#!/bin/sh
set -eu

IMAGE=${1:?usage: container-smoke.sh <image>}
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
MONGO=leanote-container-smoke-mongo
APP=leanote-container-smoke-app
NETWORK=leanote-container-smoke
TMP_HEALTH=$(mktemp)
TMP_CONFIG=$(mktemp)
FILES_DIR=$(mktemp -d)
UPLOAD_DIR=$(mktemp -d)
chmod 0777 "$FILES_DIR" "$UPLOAD_DIR"
cleanup() {
  status=$?
  set +e
  cleanup_error=0
  if docker inspect "$APP" >/dev/null 2>&1; then docker rm -f "$APP" >/dev/null || cleanup_error=1; fi
  if docker inspect "$MONGO" >/dev/null 2>&1; then docker rm -f "$MONGO" >/dev/null || cleanup_error=1; fi
  if docker network inspect "$NETWORK" >/dev/null 2>&1; then docker network rm "$NETWORK" >/dev/null || cleanup_error=1; fi
  rm -f "$TMP_HEALTH" "$TMP_HEALTH.headers" "$TMP_HEALTH.pdf.headers" "$TMP_HEALTH.pdf.html" "$TMP_CONFIG"
  rm -rf "$FILES_DIR" "$UPLOAD_DIR"
  test ! -e "$TMP_HEALTH" && test ! -e "$TMP_CONFIG" || cleanup_error=1
  if [ "$status" -eq 0 ] && [ "$cleanup_error" -ne 0 ]; then status=1; fi
  exit "$status"
}
trap cleanup EXIT INT TERM
docker rm -f "$APP" "$MONGO" >/dev/null 2>&1 || true
docker network create "$NETWORK" >/dev/null
docker run -d --name "$MONGO" --network "$NETWORK" \
  --health-cmd 'mongosh --quiet --eval "db.runCommand({ping:1}).ok"' \
  --health-interval 2s --health-timeout 2s --health-retries 30 \
  docker.io/library/mongo:8.0@sha256:376f5173003b5408d7b8e6989667231c0bf0cefdce379d7c814910429d1a7a85 >/dev/null
deadline=$(($(date +%s) + 90))
while [ "$(docker inspect -f '{{.State.Health.Status}}' "$MONGO")" != healthy ]; do
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'MongoDB readiness timeout' >&2; exit 1; }
  sleep 1
done
docker cp "$ROOT/mongodb_backup/leanote_install_data" "$MONGO:/leanote_install_data"
docker exec "$MONGO" mongorestore --db leanote --dir /leanote_install_data --drop >/dev/null
printf '%s\n' '[prod]' 'db.urlEnv=${MONGODB_URL}' 'db.dbname=leanote' 'app.secret=${LEANOTE_APP_SECRET}' 'http.addr=0.0.0.0' 'http.port=9000' > "$TMP_CONFIG"
chmod 0440 "$TMP_CONFIG"
docker run -d --name "$APP" --user 10001:10001 --group-add "$(id -g)" --network "$NETWORK" -p 9000:9000 \
  -v "$TMP_CONFIG:/etc/leanote/app.conf:ro" -v "$FILES_DIR:/app/files" -v "$UPLOAD_DIR:/app/public/upload" \
  -e MONGODB_URL="mongodb://$MONGO:27017/leanote" \
  -e LEANOTE_APP_SECRET='container-smoke-secret-012345678901234567890' "$IMAGE" >/dev/null
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
  *) echo 'CONTAINER_SMOKE_PDF_URL must target the real /note/toPdf route' >&2; exit 1 ;;
esac
curl -fsS -D "$TMP_HEALTH.pdf.headers" -o "$TMP_HEALTH.pdf.html" "$CONTAINER_SMOKE_PDF_URL"
grep -Eiq '^Content-Type: text/html(;|$)' "$TMP_HEALTH.pdf.headers"
test -s "$TMP_HEALTH.pdf.html"
docker exec "$APP" env "CONTAINER_SMOKE_PDF_URL=$CONTAINER_SMOKE_PDF_URL" sh -c 'printf persisted > /app/files/smoke-marker && test -x /usr/local/bin/wkhtmltopdf && wkhtmltopdf --quiet "$CONTAINER_SMOKE_PDF_URL" /app/files/smoke.pdf'
test -s "$FILES_DIR/smoke.pdf"
test "$(dd if="$FILES_DIR/smoke.pdf" bs=1 count=5 2>/dev/null)" = '%PDF-'
printf persisted-upload > "$UPLOAD_DIR/smoke-upload"
docker restart "$APP" >/dev/null
test -f "$FILES_DIR/smoke-marker"
test "$(docker exec "$APP" cat /app/files/smoke-marker)" = persisted
test "$(docker exec "$APP" cat /app/public/upload/smoke-upload)" = persisted-upload
deadline=$(($(date +%s) + 180))
while :; do
  code=$(curl -sS -D "$TMP_HEALTH.headers" -o "$TMP_HEALTH" -w '%{http_code}' http://127.0.0.1:9000/healthz || true)
  if [ "$code" = 200 ] && grep -Fx '{"status":"ready"}' "$TMP_HEALTH" >/dev/null; then break; fi
  [ "$(date +%s)" -lt "$deadline" ] || { echo 'healthz readiness timeout after restart' >&2; exit 1; }
  sleep 1
done
