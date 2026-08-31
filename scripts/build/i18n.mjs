import fs from 'node:fs/promises';
import path from 'node:path';
import { MANIFEST, assertNoSymlinkPath, resolveRepoPath } from './manifest.mjs';

const anyGetMsg = /\bgetMsg\s*\(/g;
const msgProperty = /\bmsg\s*:/g;
const regexPrefixKeywords = new Set([
  'await', 'case', 'delete', 'do', 'else', 'in', 'instanceof', 'new',
  'of', 'return', 'throw', 'typeof', 'void', 'yield',
]);

function isRegexStart(source, index) {
  let cursor = index - 1;
  while (cursor >= 0 && /\s/.test(source[cursor])) cursor -= 1;
  if (cursor < 0 || /[([{:;,=!&|?+\-*%^~]/.test(source[cursor])) return true;
  if (!/[A-Za-z0-9_$]/.test(source[cursor])) return false;
  const end = cursor + 1;
  while (cursor >= 0 && /[A-Za-z0-9_$]/.test(source[cursor])) cursor -= 1;
  return regexPrefixKeywords.has(source.slice(cursor + 1, end));
}

function findRegexEnd(source, start) {
  let escaped = false;
  let inClass = false;
  for (let cursor = start + 1; cursor < source.length; cursor += 1) {
    const char = source[cursor];
    if (!escaped && char === '[') inClass = true;
    else if (!escaped && char === ']') inClass = false;
    else if (!escaped && char === '/' && !inClass) return cursor;
    escaped = !escaped && char === '\\';
    if (char !== '\\') escaped = false;
  }
  return -1;
}

function locator(source, index) {
  const line = source.slice(0, index).split('\n').length;
  const lineStart = source.lastIndexOf('\n', index - 1) + 1;
  return { line, column: index - lineStart + 1 };
}

function maskNonCode(source) {
  const masked = source.split('');
  const blank = (index) => { if (masked[index] !== '\n' && masked[index] !== '\r') masked[index] = ' '; };
  let state = 'code'; let quote = null; let escaped = false; let regexClass = false;
  for (let cursor = 0; cursor < source.length; cursor += 1) {
    const char = source[cursor]; const next = source[cursor + 1];
    if (state === 'html-comment') { blank(cursor); if (char === '-' && source.startsWith('-->', cursor)) { blank(cursor + 1); blank(cursor + 2); cursor += 2; state = 'code'; } continue; }
    if (state === 'line-comment') { blank(cursor); if (char === '\n') state = 'code'; continue; }
    if (state === 'block-comment') { blank(cursor); if (char === '*' && next === '/') { blank(cursor + 1); cursor += 1; state = 'code'; } continue; }
    if (state === 'string') { blank(cursor); if (!escaped && char === quote) state = 'code'; escaped = char === '\\' && !escaped; if (char !== '\\') escaped = false; continue; }
    if (state === 'regex') { blank(cursor); if (!escaped && char === '[') regexClass = true; else if (!escaped && char === ']') regexClass = false; else if (!escaped && char === '/' && !regexClass) state = 'code'; escaped = char === '\\' && !escaped; if (char !== '\\') escaped = false; continue; }
    if (char === '/' && next === '/') { blank(cursor); blank(cursor + 1); cursor += 1; state = 'line-comment'; continue; }
    if (char === '/' && next === '*') { blank(cursor); blank(cursor + 1); cursor += 1; state = 'block-comment'; continue; }
    if (source.startsWith('<!--', cursor)) { blank(cursor); blank(cursor + 1); blank(cursor + 2); blank(cursor + 3); cursor += 3; state = 'html-comment'; continue; }
    if (char === '/' && isRegexStart(source, cursor)) { blank(cursor); state = 'regex'; regexClass = false; escaped = false; continue; }
    if (char === '"' || char === "'" || char === '`') { quote = char; escaped = false; blank(cursor); state = 'string'; }
  }
  return masked.join('');
}

function templateCallIndices(source) {
  const calls = [];
  const interpolationEnd = (start) => {
    let depth = 1;
    let quote = null;
    let escaped = false;
    let lineComment = false;
    let blockComment = false;
    for (let cursor = start; cursor < source.length; cursor += 1) {
      const char = source[cursor];
      const next = source[cursor + 1];
      if (lineComment) { if (char === '\n') lineComment = false; continue; }
      if (blockComment) { if (char === '*' && next === '/') { blockComment = false; cursor += 1; } continue; }
      if (quote) {
        if (!escaped && char === quote) quote = null;
        escaped = !escaped && char === '\\';
        if (char !== '\\') escaped = false;
        continue;
      }
      if (char === '/' && next === '/') { lineComment = true; cursor += 1; continue; }
      if (char === '/' && next === '*') { blockComment = true; cursor += 1; continue; }
      if (char === '/' && isRegexStart(source, cursor)) {
        const end = findRegexEnd(source, cursor);
        if (end < 0) return source.length;
        cursor = end;
        continue;
      }
      if (char === '"' || char === "'") { quote = char; escaped = false; continue; }
      if (char === '{') depth += 1;
      else if (char === '}' && --depth === 0) return cursor;
    }
    return source.length;
  };
  const templateCalls = (start) => {
    let cursor = start + 1;
    let escaped = false;
    while (cursor < source.length) {
      const char = source[cursor];
      if (escaped) { escaped = false; cursor += 1; continue; }
      if (char === '\\') { escaped = true; cursor += 1; continue; }
      if (char === '`') return cursor;
      if (char === '$' && source[cursor + 1] === '{') {
        const bodyStart = cursor + 2;
        const bodyEnd = interpolationEnd(bodyStart);
        const body = source.slice(bodyStart, bodyEnd);
        const masked = maskNonCode(body);
        for (const match of masked.matchAll(anyGetMsg)) calls.push(bodyStart + match.index);
        cursor = bodyEnd + 1;
        continue;
      }
      cursor += 1;
    }
    return source.length;
  };
  let cursor = 0;
  while (cursor < source.length) {
    const char = source[cursor];
    const next = source[cursor + 1];
    if (char === '/' && next === '/') { const end = source.indexOf('\n', cursor + 2); cursor = end < 0 ? source.length : end + 1; continue; }
    if (char === '/' && next === '*') { const end = source.indexOf('*/', cursor + 2); cursor = end < 0 ? source.length : end + 2; continue; }
    if (source.startsWith('<!--', cursor)) { const end = source.indexOf('-->', cursor + 4); cursor = end < 0 ? source.length : end + 3; continue; }
    if (char === '"' || char === "'") { const end = findStringEnd(source, cursor, char); cursor = end < 0 ? source.length : end + 1; continue; }
    if (char === '/' && isRegexStart(source, cursor)) {
      let end = findRegexEnd(source, cursor);
      if (end < 0) return calls;
      end += 1;
      while (/[a-z]/i.test(source[end] ?? '')) end += 1;
      cursor = end;
      continue;
    }
    if (char === '`') { cursor = templateCalls(cursor) + 1; continue; }
    cursor += 1;
  }
  return calls;
}

function maskComments(source) {
  const masked = source.split('');
  const blank = (index) => { if (masked[index] !== '\n' && masked[index] !== '\r') masked[index] = ' '; };
  let state = 'code';
  let quote = null;
  let escaped = false;
  let regexClass = false;
  for (let cursor = 0; cursor < source.length; cursor += 1) {
    const char = source[cursor];
    const next = source[cursor + 1];
    if (state === 'line-comment') { blank(cursor); if (char === '\n') state = 'code'; continue; }
    if (state === 'block-comment') { blank(cursor); if (char === '*' && next === '/') { blank(cursor + 1); cursor += 1; state = 'code'; } continue; }
    if (state === 'html-comment') { blank(cursor); if (char === '-' && source.startsWith('-->', cursor)) { blank(cursor + 1); blank(cursor + 2); cursor += 2; state = 'code'; } continue; }
    if (state === 'string') { if (!escaped && char === quote) state = 'code'; escaped = char === '\\' && !escaped; if (char !== '\\') escaped = false; continue; }
    if (state === 'regex') { if (!escaped && char === '[') regexClass = true; else if (!escaped && char === ']') regexClass = false; else if (!escaped && char === '/' && !regexClass) state = 'code'; escaped = char === '\\' && !escaped; if (char !== '\\') escaped = false; continue; }
    if (char === '/' && next === '/') { blank(cursor); blank(cursor + 1); cursor += 1; state = 'line-comment'; continue; }
    if (char === '/' && next === '*') { blank(cursor); blank(cursor + 1); cursor += 1; state = 'block-comment'; continue; }
    if (source.startsWith('<!--', cursor)) { blank(cursor); blank(cursor + 1); blank(cursor + 2); blank(cursor + 3); cursor += 3; state = 'html-comment'; continue; }
    if (char === '/' && isRegexStart(source, cursor)) { state = 'regex'; regexClass = false; escaped = false; continue; }
    if (char === '"' || char === "'" || char === '`') { quote = char; escaped = false; state = 'string'; }
  }
  return masked.join('');
}



function skipTrivia(source, cursor) {
  while (cursor < source.length) {
    if (/\s/.test(source[cursor])) { cursor += 1; continue; }
    if (source.startsWith('//', cursor)) {
      const end = source.indexOf('\n', cursor + 2);
      cursor = end < 0 ? source.length : end + 1;
      continue;
    }
    if (source.startsWith('/*', cursor)) {
      const end = source.indexOf('*/', cursor + 2);
      cursor = end < 0 ? source.length : end + 2;
      continue;
    }
    break;
  }
  return cursor;
}

function findStringEnd(source, start, quote) {
  let escaped = false;
  for (let cursor = start + 1; cursor < source.length; cursor += 1) {
    const char = source[cursor];
    if (!escaped && char === quote) return cursor;
    if (!escaped && char === '\\') escaped = true;
    else escaped = false;
  }
  return -1;
}

function findCallEnd(source, start) {
  let depth = 1;
  let quote = null;
  let escaped = false;
  let lineComment = false;
  let blockComment = false;
  for (let cursor = start; cursor < source.length; cursor += 1) {
    const char = source[cursor];
    const next = source[cursor + 1];
    if (lineComment) {
      if (char === '\n') lineComment = false;
      continue;
    }
    if (blockComment) {
      if (char === '*' && next === '/') { blockComment = false; cursor += 1; }
      continue;
    }
    if (quote) {
      if (!escaped && char === quote) quote = null;
      if (!escaped && char === '\\') escaped = true;
      else escaped = false;
      continue;
    }
    if ((char === '"' || char === "'" || char === '`')) { quote = char; escaped = false; continue; }
    if (char === '/' && next === '/') { lineComment = true; cursor += 1; continue; }
    if (char === '/' && next === '*') { blockComment = true; cursor += 1; continue; }
    if (char === '/' && isRegexStart(source, cursor)) {
      const end = findRegexEnd(source, cursor);
      if (end < 0) return -1;
      cursor = end;
      continue;
    }
    if (char === '(') depth += 1;
    else if (char === ')' && --depth === 0) return cursor;
  }
  return -1;
}

function parseGetMsgCall(source, match) {
  const open = match.index + match[0].length;
  let cursor = skipTrivia(source, open);
  const quote = source[cursor];
  if (quote !== '"' && quote !== "'") return { isStatic: false };
  const stringEnd = findStringEnd(source, cursor, quote);
  if (stringEnd < 0) return { isStatic: false };
  cursor = skipTrivia(source, stringEnd + 1);
  if (source[cursor] === ')') return { isStatic: true };
  if (source[cursor] !== ',') return { isStatic: false };
  const close = findCallEnd(source, cursor + 1);
  if (close < 0) return { isStatic: false };
  // The legacy API accepts at most one optional data argument. A top-level
  // comma in the remainder indicates an unsupported/malformed call.
  let depth = 0;
  let quoteState = null;
  let escaped = false;
  for (let index = cursor + 1; index < close; index += 1) {
    const char = source[index];
    if (quoteState) {
      if (!escaped && char === quoteState) quoteState = null;
      escaped = !escaped && char === '\\';
      if (char !== '\\') escaped = false;
      continue;
    }
    if (source.startsWith('//', index)) {
      const end = source.indexOf('\n', index + 2);
      index = end < 0 ? close : end;
      continue;
    }
    if (source.startsWith('/*', index)) {
      const end = source.indexOf('*/', index + 2);
      if (end < 0 || end >= close) return { isStatic: false };
      index = end + 1;
      continue;
    }
    if (char === '/' && isRegexStart(source, index)) {
      const end = findRegexEnd(source, index);
      if (end < 0 || end >= close) return { isStatic: false };
      index = end;
      continue;
    }
    if (char === '"' || char === "'" || char === '`') { quoteState = char; continue; }
    if (char === '(' || char === '[' || char === '{') depth += 1;
    else if (char === ')' || char === ']' || char === '}') depth -= 1;
    else if (char === ',' && depth === 0) return { isStatic: false };
  }
  return { isStatic: true };
}


function parseMsgProperty(source, match) {
  const cursor = skipTrivia(source, match.index + match[0].length);
  const quote = source[cursor];
  if (quote !== '"' && quote !== "'") return null;
  const end = findStringEnd(source, cursor, quote);
  if (end < 0) return null;
  return { key: source.slice(cursor + 1, end) };
}

async function walk(root, relative) {
  const dir = assertNoSymlinkPath(root, relative, 'i18n scan root');
  const repository = await fs.realpath(root);
  let realDir;
  try { realDir = await fs.realpath(dir); } catch (error) { throw new Error(`cannot scan i18n root ${relative}: ${error.message}`); }
  if (realDir !== repository && !realDir.startsWith(`${repository}${path.sep}`)) throw new Error(`i18n scan root escapes repository: ${relative}`);
  const result = [];
  let names;
  try {
    names = await fs.readdir(dir, { withFileTypes: true });
  } catch (error) {
    throw new Error(`cannot scan i18n root ${relative}: ${error.message}`);
  }
  for (const item of names.sort((a, b) => a.name.localeCompare(b.name))) {
    const child = `${relative}/${item.name}`;
    if (item.isDirectory()) result.push(...await walk(root, child));
    else if (item.isSymbolicLink()) throw new Error(`symlink in i18n scan root: ${child}`);
    else if (/\.(?:js|html)$/.test(item.name)) result.push(child);
  }
  return result;
}


export async function scanI18nSources(root, manifest = MANIFEST) {
  const excluded = new Set(manifest.i18nDerivedInputExclusions);
  const keys = [];
  const keySources = new Map();
  const dynamic = [];
  const dynamicSeen = [];
  const files = (await Promise.all(manifest.i18nScanRoots.map((item) => walk(root, item)))).flat();
  for (const relative of [...new Set(files)].sort()) {
    if (excluded.has(relative)) continue;
    assertNoSymlinkPath(root, relative, 'i18n source');
    const source = await fs.readFile(resolveRepoPath(root, relative), 'utf8');
    const code = maskNonCode(source);
    const namespace = relative.includes('/blog/') ? 'blog' : 'msg';
    for (const match of code.matchAll(anyGetMsg)) {
      const parsed = parseGetMsgCall(source, match);
      if (!parsed.isStatic) continue;
      const location = locator(source, match.index);
      const cursor = skipTrivia(source, match.index + match[0].length);
      const stringEnd = findStringEnd(source, cursor, source[cursor]);
      const item = { key: source.slice(cursor + 1, stringEnd), namespace, path: relative, line: location.line };
      keys.push(item);
      const sourceKey = `${namespace}:${item.key}`;
      if (!keySources.has(sourceKey)) keySources.set(sourceKey, []);
      keySources.get(sourceKey).push({ path: relative, line: location.line, column: location.column });
    }
    // JavaScript strings must be blanked so text such as "msg: 'fake'" is
    // ignored. HTML validation attributes, however, carry real JSON msg keys
    // inside quoted attributes; retain those via the comment-only mask.
    const msgCode = relative.endsWith('.html') ? maskComments(source) : code;
    for (const match of msgCode.matchAll(msgProperty)) {
      const parsed = parseMsgProperty(source, match);
      if (!parsed) continue;
      // Revel template expressions embedded in validation attributes are server-side strings, not client keys.
      if (parsed.key.trimStart().startsWith('{{')) continue;
      const location = locator(source, match.index);
      const item = { key: parsed.key, namespace, path: relative, line: location.line };
      keys.push(item);
      const sourceKey = `${namespace}:${item.key}`;
      if (!keySources.has(sourceKey)) keySources.set(sourceKey, []);
      keySources.get(sourceKey).push({ path: relative, line: location.line, column: location.column });
    }
    for (const match of code.matchAll(anyGetMsg)) {
      const location = locator(source, match.index);
      const { isStatic } = parseGetMsgCall(source, match);
      if (!isStatic) {
        const known = manifest.dynamicKeyExceptions.some((item) => item.path === relative && item.line === location.line && item.column === location.column);
        if (!known) dynamic.push({ path: relative, ...location });
        else dynamicSeen.push({ path: relative, ...location });
      }
    }
    for (const index of templateCallIndices(source)) {
      const location = locator(source, index);
      const match = { index, 0: 'getMsg(' };
      const parsed = parseGetMsgCall(source, match);
      if (parsed.isStatic) {
        const cursor = skipTrivia(source, index + match[0].length);
        const stringEnd = findStringEnd(source, cursor, source[cursor]);
        const item = { key: source.slice(cursor + 1, stringEnd), namespace, path: relative, line: location.line };
        keys.push(item);
        const sourceKey = `${namespace}:${item.key}`;
        if (!keySources.has(sourceKey)) keySources.set(sourceKey, []);
        keySources.get(sourceKey).push({ path: relative, line: location.line, column: location.column });
        continue;
      }
      const known = manifest.dynamicKeyExceptions.some((item) => item.path === relative && item.line === location.line && item.column === location.column);
      if (known) dynamicSeen.push({ path: relative, ...location });
      else dynamic.push({ path: relative, ...location });
    }
  }
  // Validate and retain every manifest-registered dynamic locator even when
  // surrounding legacy syntax is unusual; the source must still contain the
  // exact call at that line/column.
  for (const exception of manifest.dynamicKeyExceptions) {
    assertNoSymlinkPath(root, exception.path, 'dynamic i18n source');
    const source = await fs.readFile(resolveRepoPath(root, exception.path), 'utf8');
    const lines = source.split(/\r?\n/);
    const line = lines[exception.line - 1] ?? '';
    const offset = exception.column - 1;
    let absolute = 0;
    for (let lineNumber = 1; lineNumber < exception.line; lineNumber += 1) {
      const newline = source.indexOf('\n', absolute);
      if (newline < 0) { absolute = source.length; break; }
      absolute = newline + 1;
    }
    absolute += offset;
    const match = { index: absolute, 0: 'getMsg(' };
    if (!/^\s*getMsg\s*\(/.test(line.slice(offset))) throw new Error(`dynamic i18n exception does not match source: ${exception.path}:${exception.line}:${exception.column}`);
    if (parseGetMsgCall(source, match).isStatic) throw new Error(`dynamic i18n exception is not dynamic: ${exception.path}:${exception.line}:${exception.column}`);
    if (!dynamicSeen.some((item) => item.path === exception.path && item.line === exception.line && item.column === exception.column)) dynamicSeen.push({ ...exception });
  }
  if (dynamic.length) throw new Error(`unregistered dynamic i18n key at ${dynamic.map((item) => `${item.path}:${item.line}:${item.column}`).join(', ')}`);
  const deduped = [...new Map(keys.map((item) => [`${item.namespace}:${item.key}:${item.path}:${item.line}`, item])).values()];
  const dynamicUnique = [...new Map(dynamicSeen.map((item) => [`${item.path}:${item.line}:${item.column}`, item])).values()].sort((a, b) => a.path.localeCompare(b.path) || a.line - b.line || a.column - b.column);
  const expectedDynamic = new Set(manifest.dynamicKeyExceptions.map((item) => `${item.path}:${item.line}:${item.column}`));
  const actualDynamic = new Set(dynamicUnique.map((item) => `${item.path}:${item.line}:${item.column}`));
  if (expectedDynamic.size !== actualDynamic.size || [...expectedDynamic].some((item) => !actualDynamic.has(item))) throw new Error(`dynamic i18n key contract changed: expected ${[...expectedDynamic].join(', ')}, got ${[...actualDynamic].join(', ')}`);
  return { keys: deduped.sort((a, b) => a.namespace.localeCompare(b.namespace) || a.key.localeCompare(b.key) || a.path.localeCompare(b.path) || a.line - b.line), dynamic: dynamicUnique, keySources };
}

export async function parseMessages(file) {
  const source = await fs.readFile(file, 'utf8');
  const result = {};
  for (const [index, raw] of source.split(/\r?\n/).entries()) {
    const line = raw.replace(/\r$/, '');
    if (!line.trim() || line.trimStart().startsWith('#')) continue;
    if (/^\s*\[[^\]]+\]\s*$/.test(line)) continue;
    const separator = line.indexOf('=');
    if (separator < 1) throw new Error(`invalid message line ${file}:${index + 1}`);
    result[line.slice(0, separator)] = line.slice(separator + 1);
  }
  return result;
}

async function localeMaps(root, locale, manifest = MANIFEST) {
  const repository = await fs.realpath(root);
  const configured = manifest.i18nMessageFiles;
  if (!Array.isArray(configured) || configured.length !== 6 || !configured.includes('msg') || !configured.includes('member') || !configured.includes('markdown') || !configured.includes('album') || !configured.includes('blog') || !configured.includes('tinymce_editor')) throw new Error('manifest.i18nMessageFiles must declare exactly six message files');
  const messageName = (input, namespace) => {
    const normalized = input.replaceAll('\\', '/');
    const prefix = `messages/${locale}/`;
    if (!normalized.startsWith(prefix) || !normalized.endsWith('.conf')) throw new Error(`manifest message input does not match locale ${locale}/${namespace}: ${input}`);
    const relative = normalized.slice(prefix.length, -'.conf'.length);
    if (!relative || relative.includes('/') || !configured.includes(relative)) throw new Error(`manifest message input is not declared for ${locale}/${namespace}: ${input}`);
    return relative;
  };
  const load = async (input, namespace) => {
    const normalized = input.replaceAll('\\', '/');
    const name = messageName(input, namespace);
    const file = resolveRepoPath(root, normalized);
    assertNoSymlinkPath(root, normalized, 'message input');
    const real = await fs.realpath(file);
    if (real !== repository && !real.startsWith(`${repository}${path.sep}`)) throw new Error(`message input escapes repository: ${normalized}`);
    return { name, messages: await parseMessages(file) };
  };
  const entries = new Map(manifest.i18n.filter((entry) => entry.locale === locale).map((entry) => [entry.namespace, entry]));
  const msgEntry = entries.get('msg');
  const blogEntry = entries.get('blog');
  const tinymceEntry = entries.get('tinymce');
  if (!msgEntry || !blogEntry || !tinymceEntry) throw new Error(`manifest i18n entries incomplete for ${locale}`);
  const expectedMsgNames = configured.filter((name) => !['blog', 'tinymce_editor'].includes(name));
  const msgNames = msgEntry.inputs.map((input) => messageName(input, 'msg'));
  if (msgNames.length !== expectedMsgNames.length || msgNames.some((name, index) => name !== expectedMsgNames[index])) {
    throw new Error(`manifest i18n message inputs incomplete for ${locale}`);
  }
  const msg = {};
  for (const input of msgEntry.inputs) Object.assign(msg, (await load(input, 'msg')).messages);
  if (blogEntry.inputs.length !== 1 || tinymceEntry.inputs.length !== 1) throw new Error(`manifest i18n namespace inputs invalid for ${locale}`);
  const blog = await load(blogEntry.inputs[0], 'blog');
  const tinymce = await load(tinymceEntry.inputs[0], 'tinymce');
  if (blog.name !== 'blog' || tinymce.name !== 'tinymce_editor') throw new Error(`manifest i18n namespace inputs invalid for ${locale}`);
  return { msg, blog: blog.messages, tinymce: tinymce.messages };
}

export async function buildI18n(root, stagingRoot, manifest = MANIFEST, fixture = null) {
  const scan = await scanI18nSources(root, manifest);
  if (fixture) {
    const expectedLocales = fixture.locales ?? [];
    if (JSON.stringify(expectedLocales) !== JSON.stringify(manifest.locales)) {
      throw new Error('i18n locale contract changed; update specification fixture first');
    }
    const expected = new Set((fixture.keys ?? []).map((item) => `${item.namespace}:${item.key}:${item.path}:${item.line}`));
    const actual = new Set(scan.keys.map((item) => `${item.namespace}:${item.key}:${item.path}:${item.line}`));
    if (expected.size !== actual.size || [...expected].some((item) => !actual.has(item))) throw new Error('i18n static key contract changed; update specification fixture first');
    const expectedDynamic = new Set((fixture.dynamic ?? []).map((item) => `${item.path}:${item.line}:${item.column}`));
    const actualDynamic = new Set(scan.dynamic.map((item) => `${item.path}:${item.line}:${item.column}`));
    if (expectedDynamic.size !== actualDynamic.size || [...expectedDynamic].some((item) => !actualDynamic.has(item))) {
      throw new Error('i18n dynamic key contract changed; update specification fixture first');
    }
  }
  const byNamespace = { msg: new Set(), blog: new Set() };
  for (const item of scan.keys) byNamespace[item.namespace].add(item.key);
  for (const locale of manifest.locales) {
    const maps = await localeMaps(root, locale, manifest);
    if (fixture) {
      const baseline = fixture.messages?.[locale];
      if (!baseline) throw new Error(`missing i18n message fixture for ${locale}`);
      for (const namespace of ['msg', 'blog', 'tinymce']) {
        const expected = baseline[namespace];
        if (!expected || JSON.stringify(Object.fromEntries(Object.entries(expected).sort())) !== JSON.stringify(Object.fromEntries(Object.entries(maps[namespace]).sort()))) {
          throw new Error(`i18n message contract changed for ${locale}/${namespace}; update specification fixture first`);
        }
      }
    }
    for (const namespace of ['msg', 'blog']) {
      const available = maps[namespace];
      const missing = [...byNamespace[namespace]].filter((key) => !(key in available));
      const baseline = fixture?.missing?.[locale]?.[namespace] ?? [];
      const unexpected = fixture ? missing.filter((key) => !baseline.includes(key)) : [];
       if (unexpected.length) {
         const details = unexpected.map((key) => {
           const source = scan.keySources?.get(`${namespace}:${key}`)?.[0];
           return source ? `${key} (${source.path}:${source.line}:${source.column})` : key;
         });
         throw new Error(`missing i18n keys for ${locale}/${namespace}: ${details.join(', ')}`);
       }
      const selected = Object.fromEntries([...byNamespace[namespace]].sort().filter((key) => key in available).map((key) => [key, available[key]]));
      const out = `var MSG=${JSON.stringify(selected)};function getMsg(key, data) {var msg = MSG[key];if(msg) {if(data) {if(!isArray(data)) {data = [data];}for(var i = 0; i < data.length; ++i) {msg = msg.replace("%s", data[i]);}}return msg;}return key;}`;
      const entry = manifest.i18n.find((item) => item.locale === locale && item.namespace === namespace);
      const outputPath = resolveRepoPath(stagingRoot, entry.output);
      await fs.mkdir(path.dirname(outputPath), { recursive: true });
      await fs.writeFile(outputPath, `${out}\n`, 'utf8');
    }
    const tinyEntry = manifest.i18n.find((item) => item.locale === locale && item.namespace === 'tinymce');
    const tinyPath = resolveRepoPath(stagingRoot, tinyEntry.output);
    await fs.mkdir(path.dirname(tinyPath), { recursive: true });
    const tinyMceLanguage = Intl.getCanonicalLocales(locale)[0];
    await fs.writeFile(tinyPath, `tinymce.addI18n(${JSON.stringify(tinyMceLanguage)},${JSON.stringify(maps.tinymce, Object.keys(maps.tinymce).sort())});\n`, 'utf8');
  }
  return scan;
}
