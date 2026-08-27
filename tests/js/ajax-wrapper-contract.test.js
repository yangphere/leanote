const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const vm = require('node:vm');

const ROOT = path.resolve(__dirname, '../..');

function loadWrapper() {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('// ajax请求返回结果后的操作');
  const end = source.indexOf('function findParents');
  assert.notEqual(start, -1, 'ajax wrapper block start found');
  assert.notEqual(end, -1, 'ajax wrapper block end found');
  const block = source.slice(start, end);
  for (const name of ['_ajaxCallback', '_ajaxFailure', '_ajax', 'ajaxGet', 'ajaxPostJson']) {
    assert.ok(block.includes(`function ${name}`), `${name} present in wrapper block`);
  }

  const captured = [];
  const alerts = [];
  const sandbox = {
    window: {},
    getMsg: (key) => key,
    alert: (message) => alerts.push(message),
    $: { ajax: (options) => { captured.push(options); return 'jqxhr-stub'; } },
  };
  sandbox.window.alert = sandbox.alert;
  vm.createContext(sandbox);
  vm.runInContext(block, sandbox);
  return { captured, alerts, sandbox };
}

test('ajaxGet success routes response objects to successFunc', () => {
  const ctx = loadWrapper();
  const successes = [];
  const failures = [];
  ctx.sandbox.ajaxGet('/api/x', {}, (ret) => successes.push(ret), (ret) => failures.push(ret));
  const options = ctx.captured.at(-1);
  assert.equal(options.type, 'GET');
  options.success({ Ok: true });
  assert.deepEqual(successes, [{ Ok: true }]);
  assert.equal(failures.length, 0);
});

test('HTTP error handlers invoke failureFunc and never successFunc', () => {
  const ctx = loadWrapper();
  const successes = [];
  const failures = [];
  ctx.sandbox.ajaxGet('/api/x', {}, (ret) => successes.push(ret), (ret) => failures.push(ret));
  const jqxhr = { status: 500, responseText: 'boom' };
  ctx.captured.at(-1).error(jqxhr);
  assert.deepEqual(failures, [jqxhr]);
  assert.equal(successes.length, 0);
});

test('HTTP error without failureFunc keeps a visible alert instead of silence', () => {
  const ctx = loadWrapper();
  ctx.sandbox.ajaxGet('/api/x', {}, () => {});
  ctx.captured.at(-1).error({ status: 404 });
  assert.deepEqual(ctx.alerts, ['error!']);
});

test('NOTLOGIN responses alert and invoke neither callback', () => {
  const ctx = loadWrapper();
  const successes = [];
  const failures = [];
  ctx.sandbox.ajaxGet('/api/x', {}, (ret) => successes.push(ret), (ret) => failures.push(ret));
  ctx.captured.at(-1).success({ Msg: 'NOTLOGIN' });
  assert.equal(successes.length, 0);
  assert.equal(failures.length, 0);
  assert.equal(ctx.alerts.length, 1);
});

test('async defaults to true and honours an explicitly passed value', () => {
  const ctx = loadWrapper();
  ctx.sandbox.ajaxGet('/a', {}, () => {});
  assert.equal(ctx.captured.at(-1).async, true, 'undefined defaults to asynchronous');
  ctx.sandbox.ajaxGet('/b', {}, () => {}, undefined, false);
  assert.equal(ctx.captured.at(-1).async, false, 'explicit false stays synchronous');
  ctx.sandbox.ajaxGet('/c', {}, () => {}, undefined, true);
  assert.equal(ctx.captured.at(-1).async, true, 'explicit true stays asynchronous');
});

test('ajaxPostJson declares dataType correctly and routes errors to failureFunc', () => {
  const ctx = loadWrapper();
  const successes = [];
  const failures = [];
  ctx.sandbox.ajaxPostJson('/api/json', { a: 1 }, (ret) => successes.push(ret), (ret) => failures.push(ret));
  const options = ctx.captured.at(-1);
  assert.equal(options.type, 'POST');
  assert.equal(options.dataType, 'json');
  assert.equal(options.contentType, 'application/json; charset=utf-8');
  options.success({ Ok: true });
  assert.deepEqual(successes, [{ Ok: true }]);
  options.error({ status: 502 });
  assert.equal(failures.length, 1);
  assert.equal(successes.length, 1);
});
