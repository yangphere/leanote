import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { assertImageTagFormat, compareImageVersions } from './version.mjs';

const MAX_JSON_BYTES = 1024 * 1024;
const VERSION_LABEL = 'org.opencontainers.image.version';

function requireObject(value, message) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(message);
  return value;
}

function readJsonFile(filePath, description) {
  const stat = fs.statSync(filePath);
  if (!stat.isFile() || stat.size === 0 || stat.size > MAX_JSON_BYTES) {
    throw new Error(`${description} must be a nonempty regular file no larger than 1 MiB`);
  }
  try {
    return JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (error) {
    if (error instanceof SyntaxError) throw new Error(`${description} contains invalid JSON`);
    throw error;
  }
}

function validateListing(listing, expectedRepository) {
  const value = requireObject(listing, 'Skopeo tag listing must be an object');
  if (typeof expectedRepository !== 'string' || expectedRepository.length === 0) {
    throw new Error('expected repository is required');
  }
  if (value.Repository !== expectedRepository) {
    throw new Error(`Skopeo repository identity mismatch: expected ${expectedRepository}`);
  }
  if (!Array.isArray(value.Tags)) throw new Error('Skopeo Tags must be an array');
  if (!value.Tags.every((tag) => typeof tag === 'string')) {
    throw new Error('Skopeo Tags must contain only strings');
  }
  return value.Tags;
}

export function hasLatestTag(listing, expectedRepository) {
  return validateListing(listing, expectedRepository).includes('latest');
}

export function shouldPromoteLatest({ candidateVersion, listing, expectedRepository, latestConfig }) {
  const latestExists = hasLatestTag(listing, expectedRepository);
  if (!latestExists) {
    assertImageTagFormat(candidateVersion);
    return true;
  }
  if (latestConfig === undefined) throw new Error('latest config is required when the latest tag exists');
  const configDocument = requireObject(latestConfig, 'latest config must be an object');
  const config = requireObject(configDocument.config, 'latest config.config must be an object');
  const labels = requireObject(config.Labels, 'latest config Labels must be an object');
  const currentVersion = labels[VERSION_LABEL];
  if (typeof currentVersion !== 'string' || currentVersion.length === 0) {
    throw new Error(`latest version label is missing: ${VERSION_LABEL}`);
  }
  return compareImageVersions(candidateVersion, currentVersion) > 0;
}

export function latestPresenceFromFile(listingPath, expectedRepository) {
  return hasLatestTag(readJsonFile(listingPath, 'Skopeo tag listing'), expectedRepository);
}

export function latestPromotionFromFiles(listingPath, configPath, expectedRepository, candidateVersion) {
  const listing = readJsonFile(listingPath, 'Skopeo tag listing');
  const latestConfig = hasLatestTag(listing, expectedRepository)
    ? readJsonFile(configPath, 'Skopeo latest config')
    : undefined;
  return shouldPromoteLatest({ candidateVersion, listing, expectedRepository, latestConfig });
}

if (process.argv[1] && path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url))) {
  try {
    const [mode, ...args] = process.argv.slice(2);
    let result;
    if (mode === '--has-latest' && args.length === 2) {
      result = latestPresenceFromFile(args[0], args[1]);
    } else if (mode === '--should-promote' && args.length === 4) {
      result = latestPromotionFromFiles(args[0], args[1], args[2], args[3]);
    } else {
      throw new Error('usage: check-latest-promotion.mjs --has-latest <tags.json> <repository> | --should-promote <tags.json> <config.json> <repository> <candidate-version>');
    }
    process.stdout.write(`${result}\n`);
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
