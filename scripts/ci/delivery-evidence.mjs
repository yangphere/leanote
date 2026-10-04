import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import crypto from 'node:crypto';
import { execFile, execFileSync } from 'node:child_process';
import { promisify } from 'node:util';
import { fileURLToPath } from 'node:url';
import { executionIdentity, assertExecutionIdentity } from './quality-contract.mjs';
import { ReleaseError, describeReleaseError } from '../release-error.mjs';

const execute = promisify(execFile);
const digest = (value) => crypto.createHash('sha256').update(value).digest('hex');
const objectDigest = (value) => digest(JSON.stringify(value));
const sha = /^[0-9a-f]{64}$/;
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:/-]{0,199}$/;
const owners = ['domain-contracts', 'application-identity', 'application-notes', 'application-content', 'application-publishing', 'application-admin', 'infrastructure-persistence', 'interface-http', 'presentation-frontend'];

function keys(value, expected, label) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).sort().join(',') !== [...expected].sort().join(',')) throw new ReleaseError(label + ' schema mismatch');
}
function requireValue(ok, message) { if (!ok) throw new ReleaseError(message); }
async function boundedJSON(file, maxBytes = 4 * 1024 * 1024) {
  const stat = await fs.lstat(file);
  requireValue(stat.isFile() && !stat.isSymbolicLink() && stat.size <= maxBytes, 'invalid evidence file');
  return JSON.parse(await fs.readFile(file, 'utf8'));
}

export function deliveryIdentity(env) {
  const identity = executionIdentity(env);
  requireValue(typeof env.GITHUB_REPOSITORY === 'string' && new RegExp('^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$').test(env.GITHUB_REPOSITORY), 'trusted repository provenance invalid');
  return { repository: env.GITHUB_REPOSITORY, ...identity };
}

export async function buildDeliveryCatalog(root) {
  const scenarios = [];
  const sources = [];
  const archived = await fs.readdir(path.join(root, '.trellis/tasks/archive'), { withFileTypes: true });
  const locations = ['.trellis/tasks', ...archived.filter((entry) => entry.isDirectory() && /^[0-9]{4}-[0-9]{2}$/.test(entry.name)).map((entry) => '.trellis/tasks/archive/' + entry.name)];
  const taskRoots = new Map();
  for (const owner of [...owners, 'delivery-verification']) {
    const matches = [];
    for (const location of locations) {
      const relative = location + '/09-08-' + owner;
      try {
        const stat = await fs.lstat(path.join(root, relative));
        requireValue(stat.isDirectory() && !stat.isSymbolicLink(), 'task source must be a regular directory');
        matches.push(relative);
      } catch (error) { if (error.code !== 'ENOENT') throw error; }
    }
    requireValue(matches.length === 1, 'task source must resolve uniquely: ' + owner);
    taskRoots.set(owner, matches[0]);
  }
  async function source(owner, relative) {
    const bytes = await fs.readFile(path.join(root, relative));
    sources.push({ owner, path: relative, sha256: digest(bytes) });
    return bytes.toString('utf8');
  }
  function add(id, owner, reference, clause) { scenarios.push({ id, owner, source: reference, clause }); }
  for (const owner of owners) {
    const relative = taskRoots.get(owner) + '/acceptance/evidence-matrix.md';
    const body = await source(owner, relative);
    const found = new Set();
    for (const line of body.split(String.fromCharCode(10))) {
      if (['- [ ] ', '- [x] ', '- [X] '].some((prefix) => line.startsWith(prefix))) {
        const clause = 'manual-' + digest(line.slice(6).trim()).slice(0, 16);
        add(owner + ':' + clause, owner, relative, clause);
      }
      if (!line.startsWith('|')) continue;
      const label = line.split('|')[1].trim();
      const match = /^(AC-[A-Z]+[0-9]+(?:-[A-Z]+)*|E-C[0-9]+)/.exec(label);
      if (!match) continue;
      const clause = match[1];
      // The two AC-PF5 rows have distinct approved scopes; keep both.
      const id = owner + ':' + clause + (found.has(clause) ? ':' + digest(label).slice(0, 8) : '');
      found.add(clause); add(id, owner, relative, clause);
    }
    requireValue(found.size > 0, 'missing upstream criteria: ' + owner);
  }
  const inventoryPath = taskRoots.get('application-notes') + '/research/action-inventory.md';
  const inventory = await source('application-notes', inventoryPath);
  const actions = new Set();
  for (const line of inventory.split(String.fromCharCode(10))) {
    const match = /^(?:WN|WH|WB|WT|AN|AB|AT)-[0-9]+$/.exec(line.startsWith('|') ? line.split('|')[1].trim() : '');
    if (match) actions.add(match[0]);
  }
  requireValue(actions.size === 38, 'authoritative notes action set must contain 38 IDs');
  for (const id of [...actions].sort()) add('notes-action:' + id, 'application-notes', inventoryPath, id);
  const taskPath = taskRoots.get('delivery-verification') + '/prd.md';
  await source('delivery', taskPath);
  for (const topology of ['mongo7-standalone', 'mongo8-standalone', 'mongo8-replica-set']) add('environment:' + topology, 'delivery', taskPath, 'R2');
  for (const stage of ['claim', 'apply', 'verify', 'save', 'commit']) add('recovery:kill-restart-' + stage, 'delivery', taskPath, 'R4');
  for (const name of ['mongo-failpoint', 'cross-host-filesystem', 'persistent-volume', 'legacy-create-repair']) add('recovery:' + name, 'delivery', taskPath, 'R4');
  for (const name of ['smtp-handoff', 'backup-restore', 'upgrade-restart']) add('integration:' + name, 'delivery', taskPath, 'R5');
  for (const name of ['production-config', 'nonroot-volumes-restart', 'package', 'pdf-readable', 'pdf-local-file-deny', 'pdf-zero-outbound', 'pdf-timeout-cleanup']) add('linux:' + name, 'delivery', taskPath, 'R7');
  for (const product of ['chrome', 'edge', 'firefox', 'safari']) {
    for (const slot of ['current_major', 'previous_major']) add('browser-business:' + product + ':' + slot, 'delivery', taskPath, 'R8');
  }
  add('compatibility:D-H6', 'interface-http', taskPath, 'R6');
  add('compatibility:MOD-004', 'interface-http', taskPath, 'R4');
  requireValue(new Set(scenarios.map((s) => s.id)).size === scenarios.length, 'duplicate required scenario');
  return { schema_version: 'leanote.delivery-catalog.v1', sources, scenarios };
}

export function validateDeliveryEvidence(value, catalog, expected) {
  keys(value, ['schema_version', 'repository', 'commit', 'ref', 'workflow', 'run', 'catalog_sha256', 'runner', 'started_at', 'finished_at', 'exit_code', 'cleanup_status', 'records'], 'delivery evidence');
  requireValue(value.schema_version === 'leanote.delivery-evidence.v1', 'delivery schema version mismatch');
  keys(value.run, ['id', 'attempt'], 'delivery run');
  requireValue(value.repository === expected.repository, 'delivery repository provenance mismatch');
  assertExecutionIdentity(value, expected);
  requireValue(value.catalog_sha256 === objectDigest(catalog), 'delivery catalog drift');
  keys(value.runner, ['executable_sha256', 'arguments_sha256'], 'delivery runner');
  requireValue(sha.test(value.runner.executable_sha256) && sha.test(value.runner.arguments_sha256), 'delivery runner provenance invalid');
  requireValue(typeof value.started_at === 'string' && typeof value.finished_at === 'string' && value.started_at.endsWith('Z') && value.finished_at.endsWith('Z') && Number.isFinite(Date.parse(value.started_at)) && Date.parse(value.finished_at) >= Date.parse(value.started_at), 'delivery execution time invalid');
  requireValue(value.exit_code === 0 && value.cleanup_status === 'passed', 'delivery execution or cleanup failed');
  requireValue(Array.isArray(value.records) && value.records.length === catalog.scenarios.length, 'delivery scenario coverage missing');
  const required = new Set(catalog.scenarios.map((s) => s.id));
  for (const record of value.records) {
    keys(record, ['id', 'status', 'discovered', 'executed', 'passed', 'failed', 'skipped', 'exit_code', 'cleanup_status', 'assertions', 'environment', 'fixture_sha256'], 'delivery scenario');
    requireValue(required.delete(record.id), 'unknown or duplicate delivery scenario');
    requireValue(['discovered', 'executed', 'passed', 'failed', 'skipped'].every((k) => Number.isSafeInteger(record[k]) && record[k] >= 0), 'delivery counts invalid');
    requireValue(record.status === 'passed' && record.discovered > 0 && record.executed === record.discovered && record.passed === record.executed && record.failed === 0 && record.skipped === 0, 'delivery scenario incomplete');
    requireValue(record.exit_code === 0 && record.cleanup_status === 'passed', 'delivery scenario exit or cleanup failed');
    requireValue(Array.isArray(record.assertions) && record.assertions.length > 0 && record.assertions.length <= 1000 && record.assertions.every((x) => typeof x === 'string' && identifier.test(x)) && new Set(record.assertions).size === record.assertions.length, 'delivery assertion identifiers invalid');
    requireValue(typeof record.environment === 'string' && identifier.test(record.environment) && sha.test(record.fixture_sha256), 'delivery environment or fixture invalid');
  }
  requireValue(required.size === 0, 'delivery scenario coverage missing');
  return value;
}

export async function runDeliveryEvidence({ root = process.cwd(), output = path.join(root, 'delivery-results'), env = process.env, trace = {} } = {}) {
  trace.stage = 'configuration';
  trace.command_exit_code = null;
  trace.cleanup_status = 'not_run';
  const identity = deliveryIdentity(env);
  requireValue(env.LEANOTE_DELIVERY_RUNNER_CONFIG && path.isAbsolute(env.LEANOTE_DELIVERY_RUNNER_CONFIG), 'protected delivery runner config required');
  const config = await boundedJSON(env.LEANOTE_DELIVERY_RUNNER_CONFIG, 65536);
  keys(config, ['executable', 'sha256', 'args', 'timeout_ms'], 'protected runner config');
  requireValue(path.isAbsolute(config.executable) && sha.test(config.sha256) && Array.isArray(config.args) && config.args.every((a) => typeof a === 'string' && !a.includes(String.fromCharCode(0))) && Number.isSafeInteger(config.timeout_ms) && config.timeout_ms > 0 && config.timeout_ms <= 7200000, 'protected runner config invalid');
  requireValue(digest(await fs.readFile(config.executable)) === config.sha256, 'protected runner executable digest mismatch');
  const commit = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim();
  requireValue(commit === identity.commit, 'delivery checkout candidate mismatch');
  requireValue(execFileSync('git', ['status', '--porcelain'], { cwd: root, encoding: 'utf8' }).trim() === '', 'delivery requires clean candidate');
  const catalog = await buildDeliveryCatalog(root);
  const work = await fs.mkdtemp(path.join(os.tmpdir(), 'leanote-delivery-run-'));
  const startedAt = new Date().toISOString();
  let result;
  let failure;
  try {
    trace.stage = 'protected-execution';
    trace.cleanup_status = 'unknown';
    const catalogPath = path.join(work, 'catalog.json');
    const resultPath = path.join(work, 'result.json');
    await fs.writeFile(catalogPath, JSON.stringify(catalog));
    try {
      await execute(config.executable, [...config.args, '--catalog', catalogPath, '--result', resultPath], { cwd: root, env, timeout: config.timeout_ms, maxBuffer: 1024 * 1024, windowsHide: true });
    } catch (error) {
      trace.command_exit_code = Number.isInteger(error.code) ? error.code : null;
      trace.process_failure = error.killed ? 'timeout-or-output-limit' : (error.signal ? 'terminated' : 'nonzero-or-start-failed');
      throw new ReleaseError('protected delivery execution failed');
    }
    trace.command_exit_code = 0;
    trace.stage = 'result-validation';
    const report = await boundedJSON(resultPath);
    keys(report, ['cleanup_status', 'records'], 'protected delivery result');
    trace.cleanup_status = ['passed', 'failed', 'unknown', 'not_run'].includes(report.cleanup_status) ? report.cleanup_status : 'unknown';
    result = { schema_version: 'leanote.delivery-evidence.v1', ...identity, catalog_sha256: objectDigest(catalog), runner: { executable_sha256: config.sha256, arguments_sha256: objectDigest(config.args) }, started_at: startedAt, finished_at: new Date().toISOString(), exit_code: 0, cleanup_status: report.cleanup_status, records: report.records };
    validateDeliveryEvidence(result, catalog, identity);
  } catch (error) { failure = error; }
  try { await fs.rm(work, { recursive: true, force: true }); }
  catch {
    trace.workspace_cleanup_status = 'failed';
    if (failure) throw failure;
    throw new ReleaseError('delivery evidence workspace cleanup failed');
  }
  trace.workspace_cleanup_status = 'passed';
  if (failure) throw failure;
  await fs.mkdir(output, { recursive: true });
  requireValue((await fs.readdir(output)).length === 0, 'delivery output must be empty');
  await fs.writeFile(path.join(output, 'catalog.json'), JSON.stringify(catalog));
  await fs.writeFile(path.join(output, 'delivery-evidence.json'), JSON.stringify(result));
  return result;
}

export async function validateDeliveryDirectory(directory, root, expected) {
  const names = (await fs.readdir(directory)).sort();
  requireValue(names.join(',') === 'catalog.json,delivery-evidence.json', 'delivery artifact allowlist mismatch');
  const catalog = await buildDeliveryCatalog(root);
  const stored = await boundedJSON(path.join(directory, 'catalog.json'));
  requireValue(objectDigest(stored) === objectDigest(catalog), 'delivery artifact catalog mismatch');
  return validateDeliveryEvidence(await boundedJSON(path.join(directory, 'delivery-evidence.json')), catalog, expected);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const trace = {};
  try {
    if (process.argv[2] === '--validate') await validateDeliveryDirectory(process.argv[3], process.cwd(), deliveryIdentity(process.env));
    else await runDeliveryEvidence({ output: process.argv[2] || path.resolve('delivery-results'), trace });
    process.stdout.write('delivery evidence validated' + String.fromCharCode(10));
  } catch (error) {
    let execution = null;
    try { execution = deliveryIdentity(process.env); } catch { /* Invalid identity is itself a failure; never invent one. */ }
    const failure = { schema_version: 'leanote.delivery-failure.v1', execution, ...trace, failure: describeReleaseError(error), recorded_at: new Date().toISOString() };
    if (process.env.DELIVERY_FAILURE_PATH) await fs.writeFile(process.env.DELIVERY_FAILURE_PATH, JSON.stringify(failure));
    process.stderr.write(JSON.stringify(failure) + String.fromCharCode(10));
    process.exitCode = 1;
  }
}
