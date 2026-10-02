const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

const root = path.resolve(__dirname, '..', '..');
const read = (relativePath) => fs.readFileSync(path.join(root, relativePath), 'utf8');
const responsiveBlock = (relativePath) => {
  const source = read(relativePath);
  const marker = '/* Responsive management shells and group cards */';
  const start = source.lastIndexOf(marker);
  assert.notEqual(start, -1, `${relativePath} is missing the responsive block`);
  return source.slice(start);
};

test('admin and member shells expose mobile navigation', () => {
  const admin = read('app/views/admin/top.html');
  const adminNav = read('app/views/admin/nav.html');
  const member = read('app/views/member/top.html');

  assert.match(admin, /class="lang-\{\{\$\.locale\}\} admin-shell"/);
  assert.match(member, /class="lang-\{\{\$\.locale\}\} member-shell"/);
  assert.doesNotMatch(admin, /nav navbar-nav ms-auto m-n d-none d-sm-block nav-user/);
  assert.doesNotMatch(admin, /aside[^>]*d-none d-sm-block/);
  assert.doesNotMatch(adminNav, /<nav[^>]*d-none d-sm-block/);
  assert.doesNotMatch(member, /nav navbar-nav ms-auto m-n d-none d-sm-block nav-user/);
});

test('group rendering branches share the responsive card class', () => {
  const source = read('app/views/member/group/index.html');
  assert.equal((source.match(/group-grid-item each-group/g) || []).length, 2);
  assert.doesNotMatch(source, /\.group-title[\s\S]*width:\s*200px/);
  for (const selector of ['.group-title', '.add-user-input', '.delete-group', '.delete-user', '#groups']) {
    assert.match(source, new RegExp(selector.replace(/[.#]/g, '\\$&')));
  }
});

test('responsive LESS and CSS blocks stay byte-for-byte synchronized', () => {
  for (const area of ['admin', 'member']) {
    const lessBlock = responsiveBlock(`public/${area}/css/${area}.less`);
    assert.equal(
      lessBlock,
      responsiveBlock(`public/${area}/css/${area}.css`),
      `${area} responsive LESS/CSS blocks drifted`,
    );
    assert.doesNotMatch(lessBlock, /overflow-x:\s*hidden/);
    assert.match(lessBlock, /header \+ section[\s\S]*padding-top:\s*0/);
    assert.match(lessBlock, /\.nav-primary > ul\.nav[\s\S]*display:\s*block/);
    assert.match(lessBlock, /\.nav-primary > ul\.nav > li > a[\s\S]*display:\s*block/);
  }
});
