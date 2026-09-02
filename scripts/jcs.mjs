import crypto from 'node:crypto';

// RFC 8785 (JCS) canonicalization restricted to the release-evidence payload
// domain: ASCII-printable strings and non-negative safe integers. Anything
// outside that domain is rejected instead of silently serialized with
// non-canonical escaping or number formatting.
function assertDomainString(value) {
  if (!/^[\x20-\x7E]*$/.test(value)) {
    throw new Error('JCS canonicalization supports ASCII-printable strings only');
  }
}

function canonicalizePrimitive(value) {
  if (typeof value === 'string') {
    assertDomainString(value);
    return JSON.stringify(value);
  }
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value) || value < 0) {
      throw new Error('JCS canonicalization supports non-negative safe integers only');
    }
    return String(value);
  }
  throw new Error(`JCS canonicalization does not support ${value === null ? 'null' : typeof value}`);
}

export function canonicalize(value) {
  if (Array.isArray(value)) return `[${value.map((item) => canonicalize(item)).join(',')}]`;
  if (value && typeof value === 'object') {
    // Array.prototype.sort compares by UTF-16 code units, which is exactly the
    // property ordering RFC 8785 §3.2.3 requires.
    const keys = Object.keys(value).sort();
    return `{${keys.map((key) => `${canonicalizePrimitive(key)}:${canonicalize(value[key])}`).join(',')}}`;
  }
  return canonicalizePrimitive(value);
}

export function jcsSha256(value) {
  return crypto.createHash('sha256').update(canonicalize(value), 'utf8').digest('hex');
}
