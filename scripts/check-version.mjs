import { readProjectVersion, assertImageTag, assertReleaseTag } from './version.mjs';

try {
  const version = readProjectVersion();
  const tag = process.env.RELEASE_TAG || process.env.GITHUB_REF_NAME;
  const mode = process.argv[2];
  if (mode === '--image-tag') {
    assertImageTag(tag, version);
  } else if (mode) {
    throw new Error(`unknown version check mode: ${mode}`);
  } else if (tag) {
    assertReleaseTag(tag, version);
  }
  if (process.env.GO_VERSION && process.env.GO_VERSION === 'dev') throw new Error('Go linker version is dev; release builds require injected X.Y.Z');
  process.stdout.write(`${version}\n`);
} catch (error) {
  console.error(error.message);
  process.exitCode = 1;
}
