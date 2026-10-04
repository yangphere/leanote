# Buildx load/push digest diagnosis

## Scope and versions

This diagnosis used only a temporary local registry and builder created for
the investigation. It did not move the `2.0.1` tag or write GHCR.

- Docker Desktop / Engine: `29.8.1`
- Docker Buildx: `0.37.1` (`0b265a9f62db554fa9aba6dd19e1bd5704bc7d8a`)
- Docker driver BuildKit: `0.33.0`
- Docker-container driver BuildKit: `0.33.1`
- Skopeo: `1.22.3` (`92397d5ba399784c6e2f9f229a4244b724743dba`)
- Pinned Skopeo test image: `quay.io/skopeo/stable@sha256:249b92db7297e5c801e19172dbb3b56fde88094a49740a5ededac8c2958bf2c0`

Ubuntu 22.04's repository supplied Skopeo `1.4.1`; a direct test failed with
`unknown flag: --preserve-digests`. The workflow therefore uses the pinned,
tested Skopeo `1.22.3` container instead of installing the runner's old and
mutable distribution package.

The fixture was a one-layer image built from
`%TEMP%\leanote-digest-repro`. Registry writes targeted the temporary
`registry:2` container on `127.0.0.1:5001` only.

## Reproduced boundary

With the default Docker driver, `buildx build --load` metadata reported the
config digest as `containerimage.digest`:

```text
containerimage.digest        sha256:7e0df15eea5755fedcabf117c26b35ab73c04e63df28d1ffeb47a2eb48d42b24
containerimage.config.digest sha256:7e0df15eea5755fedcabf117c26b35ab73c04e63df28d1ffeb47a2eb48d42b24
```

Pushing the loaded image with dockerd produced Docker schema 2 manifest
`sha256:1b08bb880c0e9b48581ff76774e674841a315a0575c69b14881ed1220eb9d52b`.
A direct BuildKit registry export of the same fixture reported and stored that
same manifest digest while retaining config `7e0df15e...`.

The docker-container driver reproduced the GitHub runner behavior more
directly. Its `--load` export reported OCI manifest
`sha256:654cfb2069694e882c2bb43800c3209d53ca13fc88099b0c05f1b18ad3be4fdf`
and config
`sha256:6ace465805c3a09ed228d20589f5e70c07e4dfb7b93a9f7e37a6e68b7225fe9d`.
Dockerd then pushed a different Docker schema 2 manifest,
`sha256:7807698c...`, while retaining the same config. This matches the real
recovery run's `1550df70...` BuildKit descriptor versus `0b67446e...`
dockerd-pushed manifest.

Forcing `--output type=docker,oci-mediatypes=false` changed the BuildKit
descriptor to Docker media type with digest
`sha256:090a05e6ec9cd94d3581c92c707c37386b31f35eea221e6788651756190d3144`,
but dockerd still pushed `7807698c...`. Matching the media-type field alone
therefore does not preserve the manifest bytes across exporters.

## Digest-preserving proof

A single Buildx invocation successfully produced both the locally loaded image
used for smoke and an OCI archive:

```powershell
docker buildx build --builder leanote-digest-repro-builder `
  --platform linux/amd64 --load `
  --output type=oci,dest=$archive `
  --metadata-file $metadata `
  -t leanote-digest-repro:dual $fixture
```

Buildx metadata reported manifest `654cfb20...` and config `6ace4658...`; the
loaded Docker image ID was exactly `6ace4658...`. The archive was then copied
without conversion:

```powershell
docker run --rm --mount type=bind,source=$fixture,target=/work `
  quay.io/skopeo/stable@sha256:249b92db7297e5c801e19172dbb3b56fde88094a49740a5ededac8c2958bf2c0 `
  copy --preserve-digests --dest-tls-verify=false `
  oci-archive:/work/dual.oci.tar `
  docker://host.docker.internal:5001/repro:dual
```

The registry `Docker-Content-Digest` header, SHA-256 of the exact manifest
response bytes, and Buildx metadata all equaled `654cfb20...`. The registry
config descriptor and loaded image ID both equaled `6ace4658...`.
Redirecting pinned Skopeo `inspect --raw` directly to a file produced the
same 476 bytes and SHA-256 `654cfb20...`; the command does not append or
normalize bytes at this boundary.

## Implementation conclusion

The stable boundary is the OCI archive emitted in the same Buildx invocation
as the loaded smoke candidate. Before any write, bind the archive manifest and
config to Buildx metadata and bind its config to the loaded image ID. Publish
that archive with a digest-preserving registry copy, then hash and validate the
registry read-back. `latest` can be written from the same archive, giving the
version and moving alias identical manifest/config digests. Existing immutable
versions can be promoted by copying `repository@verified-digest` to `latest`
after exact digest/config/label/platform checks and smoke.

Do not compare a `--load` descriptor with a later dockerd serialization and do
not run a second build after smoke. Both would leave the pre-write candidate
identity unresolved.

## Local cleanup

- Removed builder `leanote-digest-repro-builder`.
- Removed container `leanote-digest-repro-registry` and its anonymous volume.
- Removed the uniquely named reproduction images and the pinned Skopeo image.
- Purged the temporary Skopeo package installed in the Ubuntu 22.04 WSL test
  distribution.
- Recursive cleanup of `%TEMP%\leanote-digest-repro` was initially rejected by
  the automatic command safety policy. The main flow subsequently verified its
  resolved absolute path, absence of reparse points/subdirectories, and exact
  20-file reproduction inventory. Explicit nonrecursive removal of those files
  and the empty directory succeeded. No user image or volume was removed.
