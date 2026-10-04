const test = require('node:test');
const assert = require('node:assert/strict');
const crypto = require('node:crypto');

test('JCS canonicalization sorts keys by UTF-16 code units without whitespace', async () => {
  const { canonicalize } = await import('../../scripts/jcs.mjs');
  assert.equal(canonicalize({ b: 1, a: 2 }), '{"a":2,"b":1}');
  assert.equal(canonicalize({ a: { c: 3, b: [1, 2] } }), '{"a":{"b":[1,2],"c":3}}');
  // Uppercase (0x42) sorts before lowercase (0x61) exactly as UTF-16 code units.
  assert.equal(canonicalize({ b: 1, B: 2, '1a': 3 }), '{"1a":3,"B":2,"b":1}');
  assert.equal(canonicalize([]), '[]');
  assert.equal(canonicalize({}), '{}');
  assert.equal(canonicalize('plain'), '"plain"');
  assert.equal(canonicalize(0), '0');
});

test('JCS canonicalization rejects values outside the evidence payload domain', async () => {
  const { canonicalize } = await import('../../scripts/jcs.mjs');
  assert.throws(() => canonicalize({ key: 'ünïcode' }), /ASCII-printable/);
  assert.throws(() => canonicalize({ key: 'line\nbreak' }), /ASCII-printable/);
  assert.throws(() => canonicalize({ count: 1.5 }), /safe integers/);
  assert.throws(() => canonicalize({ count: -1 }), /safe integers/);
  assert.throws(() => canonicalize({ count: Number.MAX_SAFE_INTEGER + 1 }), /safe integers/);
  assert.throws(() => canonicalize({ flag: true }), /does not support/);
  assert.throws(() => canonicalize({ value: null }), /does not support/);
});

test('JCS digest matches a hand-computed canonical coverage summary', async () => {
  const { canonicalize, jcsSha256 } = await import('../../scripts/jcs.mjs');
  const summary = {
    browser_product: 'chrome',
    release_slot: 'current_major',
    items: [{
      discovered_count: 3,
      entrypoints: ['/note', '/blog'],
      executed_count: 3,
      id: 'business-flows',
      iframes: [],
      result: 'passed',
    }],
  };
  const expectedCanonical = '{"browser_product":"chrome","items":[{"discovered_count":3,"entrypoints":["/note","/blog"],'
    + '"executed_count":3,"id":"business-flows","iframes":[],"result":"passed"}],"release_slot":"current_major"}';
  assert.equal(canonicalize(summary), expectedCanonical);
  assert.equal(
    jcsSha256(summary),
    crypto.createHash('sha256').update(expectedCanonical, 'utf8').digest('hex'),
  );
});

test('JCS digest changes when any summary field changes', async () => {
  const { jcsSha256 } = await import('../../scripts/jcs.mjs');
  const base = {
    browser_product: 'edge',
    release_slot: 'previous_major',
    items: [{ discovered_count: 2, entrypoints: ['/note'], executed_count: 2, id: 'editor-flows', iframes: [], result: 'passed' }],
  };
  const variants = [
    { ...base, browser_product: 'chrome' },
    { ...base, release_slot: 'current_major' },
    { ...base, items: [{ ...base.items[0], discovered_count: 3 }] },
    { ...base, items: [{ ...base.items[0], executed_count: 1 }] },
    { ...base, items: [{ ...base.items[0], entrypoints: ['/member/blog'] }] },
    { ...base, items: [{ ...base.items[0], iframes: ['/tinymce/plugins/leaui_image/index.html'] }] },
  ];
  const baseDigest = jcsSha256(base);
  for (const variant of variants) {
    assert.notEqual(jcsSha256(variant), baseDigest);
  }
});
