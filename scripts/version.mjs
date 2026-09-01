import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const semver = /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/;

export function readProjectVersion(root = process.cwd()) {
  const pkg = JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8'));
  const lock = JSON.parse(fs.readFileSync(path.join(root, 'package-lock.json'), 'utf8'));
  if (typeof pkg.version !== 'string' || !semver.test(pkg.version)) {
    throw new Error('package.json version must be strict X.Y.Z');
  }
  if (lock.packages?.['']?.version !== pkg.version) {
    throw new Error('package-lock.json root version does not match package.json');
  }
  return pkg.version;
}

export function assertReleaseTag(tag, version) {
  if (typeof tag !== 'string' || !/^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/.test(tag)) {
    throw new Error(`release tag must match vX.Y.Z: ${tag ?? ''}`);
  }
  if (tag.slice(1) !== version) throw new Error('release tag does not match package version');
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  try {
    const version = readProjectVersion();
    if (process.argv[2]) assertReleaseTag(process.argv[2], version);
    process.stdout.write(`${version}\n`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
