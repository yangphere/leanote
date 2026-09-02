#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
OUT_DIR=${OUTPUT_DIR:-"$ROOT/dist"}
VERSION=$(node "$ROOT/scripts/version.mjs")
# The tag assertion only applies to real tag contexts: an explicit
# RELEASE_TAG or a refs/tags/* GITHUB_REF. Branch pushes carry GITHUB_REF_NAME
# too (e.g. "dev") and must not be treated as release tags.
TAG=${RELEASE_TAG:-}
case "${GITHUB_REF:-}" in
  refs/tags/*) TAG=${TAG:-"${GITHUB_REF:-}"}; TAG=${TAG#refs/tags/} ;;
esac
if [ -n "$TAG" ]; then
  node "$ROOT/scripts/version.mjs" "$TAG" >/dev/null
fi
if [ -n "${SOURCE_DATE_EPOCH:-}" ]; then EPOCH=$SOURCE_DATE_EPOCH; else EPOCH=$(git -C "$ROOT" show -s --format=%ct HEAD); fi
case "$EPOCH" in *[!0-9]*|'') echo "SOURCE_DATE_EPOCH must be a non-negative integer" >&2; exit 1;; esac

TMP=$(mktemp -d "${TMPDIR:-/tmp}/leanote-package.XXXXXX")
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT INT TERM
mkdir -p "$OUT_DIR" "$TMP/stage/bin"
export CGO_ENABLED=0
GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "-s -w -X github.com/yangphere/leanote/app/service.BuildVersion=$VERSION" \
  -o "$TMP/stage/bin/leanote" "$ROOT/cmd/leanote"
chmod 0755 "$TMP/stage/bin/leanote"

for dir in conf/app.conf-default conf/routes app/views messages public; do
  mkdir -p "$TMP/stage/$(dirname "$dir")"
  cp -R "$ROOT/$dir" "$TMP/stage/$dir"
done
rm -rf "$TMP/stage/public/upload"
find "$TMP/stage" -type f -print | sed "s#^$TMP/stage/##" | LC_ALL=C sort > "$TMP/files.list"
ARCHIVE="$OUT_DIR/leanote-v${VERSION}-linux-amd64.tar.gz"
tar --sort=name --mtime="@$EPOCH" --owner=0 --group=0 --numeric-owner \
  --mode='u=rwX,go=rX' -C "$TMP/stage" -cf "$TMP/archive.tar" -T "$TMP/files.list"
gzip -n -c "$TMP/archive.tar" > "$ARCHIVE"
HASH=$(sha256sum "$ARCHIVE" | awk '{print $1}')
printf '%s  %s\n' "$HASH" "$(basename "$ARCHIVE")" > "$ARCHIVE.sha256"
printf '%s\n' "$ARCHIVE"
