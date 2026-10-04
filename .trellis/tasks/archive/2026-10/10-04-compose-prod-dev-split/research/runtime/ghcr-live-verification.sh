#!/bin/bash
set -eu

test "$IMAGE_REPOSITORY" = ghcr.io/yangphere/leanote-ghcr-live-verification-20261004-84f31d
test "$GHCR_IMAGE" = yangphere/leanote-ghcr-live-verification-20261004-84f31d
EVIDENCE="$RUNNER_TEMP/evidence"
CONTEXT="$RUNNER_TEMP/fixture"
mkdir -p "$EVIDENCE" "$CONTEXT"

echo "$GH_TOKEN" | docker login ghcr.io -u "$GITHUB_ACTOR" --password-stdin
docker pull "$SKOPEO_IMAGE"
skopeo() {
  docker run --rm --network host \
    --mount "type=bind,source=$RUNNER_TEMP,target=/work" \
    --mount "type=bind,source=$HOME/.docker,target=/root/.docker,readonly" \
    "$SKOPEO_IMAGE" "$@"
}

# Snapshot public production identities to show the fixture has no production writes.
for tag in 2.0.1 latest; do
  skopeo inspect --authfile /root/.docker/config.json --raw \
    docker://ghcr.io/yangphere/leanote:"$tag" > "$EVIDENCE/production-$tag-before.json"
done

# The ordinary reviewed helper must reject bootstrap before this isolated fixture opts in.
if RELEASE_TAG=2.0.1 node scripts/check-ghcr-tag-absent.mjs > "$EVIDENCE/bootstrap-default.txt" 2>&1; then
  echo 'Default package bootstrap policy unexpectedly succeeded'
  exit 1
fi
grep -F 'GHCR initial package creation is disabled' "$EVIDENCE/bootstrap-default.txt"
ALLOW_INITIAL_PACKAGE_CREATE=true RELEASE_TAG=2.0.1 node scripts/check-ghcr-tag-absent.mjs > "$EVIDENCE/bootstrap-explicit.txt"

cat > "$CONTEXT/Dockerfile" <<'DOCKERFILE'
FROM scratch
ARG VERSION
LABEL org.opencontainers.image.version=$VERSION
LABEL org.opencontainers.image.source="https://github.com/yangphere/leanote"
COPY stamp /validation-stamp
DOCKERFILE

publish_fixture() {
  version=$1
  printf '%s\n' "$version" > "$CONTEXT/stamp"
  docker buildx build --platform linux/amd64 --provenance=false --sbom=false \
    --build-arg "VERSION=$version" \
    --output "type=oci,dest=$RUNNER_TEMP/candidate-$version.tar" \
    --metadata-file "$EVIDENCE/build-$version.json" "$CONTEXT"
  skopeo copy --preserve-digests --dest-authfile /root/.docker/config.json \
    "oci-archive:/work/candidate-$version.tar" docker://"$IMAGE_REPOSITORY:$version"
  skopeo inspect --authfile /root/.docker/config.json --raw \
    docker://"$IMAGE_REPOSITORY:$version" > "$EVIDENCE/version-$version.json"
  expected_manifest=$(node -e "const f=require('fs'); console.log(JSON.parse(f.readFileSync(process.argv[1]))['containerimage.digest'])" "$EVIDENCE/build-$version.json")
  expected_config=$(node -e "const f=require('fs'); console.log(JSON.parse(f.readFileSync(process.argv[1]))['containerimage.config.digest'])" "$EVIDENCE/build-$version.json")
  IMAGE_MANIFEST_PATH="$EVIDENCE/version-$version.json" EXPECTED_MANIFEST_DIGEST="$expected_manifest" EXPECTED_CONFIG_DIGEST="$expected_config" node scripts/verify-image-manifest.mjs
}

promote_fixture() {
  version=$1
  expected_decision=$2
  label=$3
  skopeo list-tags --authfile /root/.docker/config.json \
    docker://"$IMAGE_REPOSITORY" > "$EVIDENCE/tags-$label.json"
  has_latest=$(node scripts/check-latest-promotion.mjs --has-latest "$EVIDENCE/tags-$label.json" "$IMAGE_REPOSITORY")
  if [ "$has_latest" = true ]; then
    skopeo inspect --authfile /root/.docker/config.json --config \
      docker://"$IMAGE_REPOSITORY:latest" > "$EVIDENCE/latest-config-$label-before.json"
    skopeo inspect --authfile /root/.docker/config.json --raw \
      docker://"$IMAGE_REPOSITORY:latest" > "$EVIDENCE/latest-$label-before.json"
  fi
  decision=$(node scripts/check-latest-promotion.mjs --should-promote "$EVIDENCE/tags-$label.json" "$EVIDENCE/latest-config-$label-before.json" "$IMAGE_REPOSITORY" "$version")
  test "$decision" = "$expected_decision"
  printf '{"candidate":"%s","hasLatest":%s,"promote":%s}\n' "$version" "$has_latest" "$decision" > "$EVIDENCE/decision-$label.json"
  if [ "$decision" = true ]; then
    skopeo copy --preserve-digests --src-authfile /root/.docker/config.json --dest-authfile /root/.docker/config.json \
      docker://"$IMAGE_REPOSITORY:$version" docker://"$IMAGE_REPOSITORY:latest"
  else
    echo "Skipping fixture latest for $version after real registry comparison"
  fi
  skopeo inspect --authfile /root/.docker/config.json --raw \
    docker://"$IMAGE_REPOSITORY:latest" > "$EVIDENCE/latest-$label-after.json"
  skopeo inspect --authfile /root/.docker/config.json --config \
    docker://"$IMAGE_REPOSITORY:latest" > "$EVIDENCE/latest-config-$label-after.json"
  if [ "$decision" = true ]; then
    cmp "$EVIDENCE/version-$version.json" "$EVIDENCE/latest-$label-after.json"
  else
    cmp "$EVIDENCE/latest-$label-before.json" "$EVIDENCE/latest-$label-after.json"
  fi
}

publish_fixture 2.0.1
promote_fixture 2.0.1 true initialize
publish_fixture 2.0.2
promote_fixture 2.0.2 true advance
promote_fixture 2.0.2 false equal
publish_fixture 1.9.9
promote_fixture 1.9.9 false older

for tag in 2.0.1 latest; do
  skopeo inspect --authfile /root/.docker/config.json --raw \
    docker://ghcr.io/yangphere/leanote:"$tag" > "$EVIDENCE/production-$tag-after.json"
  cmp "$EVIDENCE/production-$tag-before.json" "$EVIDENCE/production-$tag-after.json"
done
echo 'Real GHCR fixture initialization, advancement and skip checks passed'
