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

test('NOTLOGIN responses alert and invoke only the failure callback', () => {
  const ctx = loadWrapper();
  const successes = [];
  const failures = [];
  ctx.sandbox.ajaxGet('/api/x', {}, (ret) => successes.push(ret), (ret) => failures.push(ret));
  ctx.captured.at(-1).success({ Msg: 'NOTLOGIN' });
  assert.equal(successes.length, 0);
  assert.deepEqual(failures, [{ Msg: 'NOTLOGIN' }]);
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

test('post accepts a DOM button reference for loading and reset', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('function post(');
  const end = source.indexOf('// 是否是正确的email', start);
  assert.notEqual(start, -1, 'post function found');
  assert.notEqual(end, -1, 'post function end found');
  const calls = [];
  let request;
  const button = {};
  const sandbox = {
    ajaxPost: (...args) => { request = args; },
    setButtonLoading: (...args) => calls.push(args),
    alert: () => {},
  };
  vm.createContext(sandbox);
  vm.runInContext(source.slice(start, end), sandbox);
  sandbox.post('/api/save', {}, () => {}, button);
  assert.deepEqual(calls, [[button]]);
  assert.equal(request[0], '/api/save');
  request[2]({ Ok: true });
  assert.deepEqual(calls, [[button], [button, false]]);
  request[3]({ status: 500 });
  assert.deepEqual(calls, [[button], [button, false], [button, false]]);
});

test('setButtonLoading resolves selector strings and restores the button', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('var buttonLoadingState');
  assert.notEqual(start, -1, 'button loading helper found');
  const attrs = {};
  const classes = new Set();
  const button = {
    tagName: 'BUTTON',
    innerHTML: 'Save',
    disabled: false,
    getAttribute: (name) => Object.hasOwn(attrs, name) ? attrs[name] : null,
    setAttribute: (name, value) => { attrs[name] = String(value); },
    removeAttribute: (name) => { delete attrs[name]; },
    classList: {
      contains: (name) => classes.has(name),
      add: (name) => classes.add(name),
      remove: (name) => classes.delete(name),
    },
  };
  const sandbox = { document: { querySelector: (selector) => selector === '#save' ? button : null } };
  vm.createContext(sandbox);
  vm.runInContext(source.slice(start), sandbox);
  sandbox.setButtonLoading('#save');
  assert.equal(button.disabled, true);
  assert.equal(attrs['aria-busy'], 'true');
  assert.equal(classes.has('is-loading'), true);
  sandbox.setButtonLoading('#save', false);
  assert.equal(button.disabled, false);
  assert.equal(attrs['aria-busy'], undefined);
  assert.equal(classes.has('is-loading'), false);
});

test('setButtonLoading restores original link disabled and ARIA state', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('var buttonLoadingState');
  const attrs = { 'aria-busy': 'false', 'aria-disabled': 'false', tabindex: '3' };
  const classes = new Set(['disabled']);
  const link = {
    tagName: 'A',
    innerHTML: 'Open',
    disabled: true,
    getAttribute: (name) => Object.hasOwn(attrs, name) ? attrs[name] : null,
    setAttribute: (name, value) => { attrs[name] = String(value); },
    removeAttribute: (name) => { delete attrs[name]; },
    classList: {
      contains: (name) => classes.has(name),
      add: (name) => classes.add(name),
      remove: (name) => classes.delete(name),
    },
  };
  const sandbox = { document: { querySelector: () => link } };
  vm.createContext(sandbox);
  vm.runInContext(source.slice(start), sandbox);
  sandbox.setButtonLoading(link);
  assert.equal(link.disabled, true);
  assert.equal(attrs['aria-busy'], 'true');
  assert.equal(attrs['aria-disabled'], 'true');
  assert.equal(attrs.tabindex, '-1');
  sandbox.setButtonLoading(link, false);
  assert.equal(link.disabled, true);
  assert.equal(attrs['aria-busy'], 'false');
  assert.equal(attrs['aria-disabled'], 'false');
  assert.equal(attrs.tabindex, '3');
  assert.equal(classes.has('disabled'), true);
});

test('post restores a link button after business failure and thrown callbacks', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('function post(');
  const end = source.indexOf('// 是否是正确的email', start);
  const calls = [];
  let request;
  const sandbox = {
    ajaxPost: (...args) => { request = args; },
    setButtonLoading: (...args) => calls.push(args),
    alert: () => {},
  };
  vm.createContext(sandbox);
  vm.runInContext(source.slice(start, end), sandbox);
  const link = { tagName: 'A' };
  sandbox.post('/api/save', {}, () => { throw new Error('callback failure'); }, link);
  assert.throws(() => request[2]({ Ok: false, Msg: 'rejected' }), /callback failure/);
  request[3]({ status: 500 });
  assert.deepEqual(calls, [[link], [link, false], [link, false]]);
});

test('post resets loading when the ajax adapter throws synchronously', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('function post(');
  const end = source.indexOf('// 是否是正确的email', start);
  const calls = [];
  const button = {};
  const sandbox = {
    ajaxPost: () => { throw new Error('adapter failure'); },
    setButtonLoading: (...args) => calls.push(args),
    alert: () => {},
  };
  vm.createContext(sandbox);
  vm.runInContext(source.slice(start, end), sandbox);
  assert.throws(() => sandbox.post('/api/save', {}, () => {}, button), /adapter failure/);
  assert.deepEqual(calls, [[button], [button, false]]);
});

test('Bootstrap modal wrapper preserves the legacy postShow hook', () => {
  const source = fs.readFileSync(path.join(ROOT, 'public/js/common.js'), 'utf8');
  const start = source.indexOf('function bootstrapInstance');
  assert.notEqual(start, -1, 'Bootstrap helper block found');
  const calls = [];
  const target = { nodeType: 1 };
  const modal = { show: () => calls.push('show') };
  const sandbox = {
    window: {
      bootstrap: {
        Modal: {
          getOrCreateInstance: (element, options) => {
            calls.push(['getOrCreateInstance', element, options]);
            return modal;
          },
        },
      },
      jQuery: (element) => ({
        find: (selector) => ({ hide: () => calls.push(['hide', element, selector]) }),
      }),
    },
  };
  vm.createContext(sandbox);
  vm.runInContext(source.slice(start), sandbox);
  sandbox.showBootstrapModal(target, { postShow: () => calls.push('postShow') });
  assert.deepEqual(calls.map((item) => Array.isArray(item) ? item[0] : item), [
    'getOrCreateInstance', 'show', 'postShow', 'hide',
  ]);
});
