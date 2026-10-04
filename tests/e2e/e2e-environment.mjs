// Shared E2E environment preflight (build smoke + business suites).
//
// Every E2E project must call ensureE2EIdentity() before any login or
// navigation that depends on the account. Write/route-injection gates must
// call confirmE2EIdentityFresh(), which re-requests /_test/e2e/identity and
// revalidates it against the current service (the cached preflight cannot
// prove the marker is still alive). All failures are thrown, never
// swallowed: the caller decides how to surface them.

const REQUIRED_ENV = [
  'LEANOTE_BASE_URL',
  'LEANOTE_E2E_EMAIL',
  'LEANOTE_E2E_PASSWORD',
  'LEANOTE_E2E_RUN_TOKEN',
];

let confirmed = null;

export async function ensureE2EIdentity() {
  if (confirmed) return confirmed;
  confirmed = await requestIdentity();
  return confirmed;
}

// Re-requests /_test/e2e/identity and revalidates the response against the
// current environment. The write/route-injection gate must use this fresh
// confirmation — a cached preflight cannot prove the marker is still alive
// or that the service behind the base URL has not changed.
export async function confirmE2EIdentityFresh() {
  confirmed = await requestIdentity();
  return confirmed;
}

async function requestIdentity() {
  const missing = REQUIRED_ENV.filter((name) => !process.env[name]);
  if (missing.length) {
    throw new Error(`e2e identity preflight: missing environment ${missing.join(', ')}`);
  }

  const baseUrl = process.env.LEANOTE_BASE_URL;
  let identityUrl;
  try {
    identityUrl = new URL('/_test/e2e/identity', baseUrl).href;
  } catch {
    throw new Error('e2e identity preflight: LEANOTE_BASE_URL is not a valid URL');
  }

  let response;
  try {
    response = await fetch(identityUrl, { signal: AbortSignal.timeout(10_000) });
  } catch (error) {
    throw new Error(`e2e identity preflight: service unreachable (${error.cause?.code || error.name})`);
  }
  if (!response.ok) {
    throw new Error(`e2e identity preflight: identity endpoint answered HTTP ${response.status}`);
  }
  const body = await response.json().catch(() => {
    throw new Error('e2e identity preflight: identity endpoint returned invalid JSON');
  });
  assertIdentityResponse(body, process.env.LEANOTE_E2E_RUN_TOKEN);

  return {
    baseUrl: baseUrl.replace(/\/$/, ''),
    email: process.env.LEANOTE_E2E_EMAIL,
    password: process.env.LEANOTE_E2E_PASSWORD,
    runToken: process.env.LEANOTE_E2E_RUN_TOKEN,
  };
}

// Pure fail-closed comparison; exported so regression tests can prove that a
// run token mismatch (digest mismatch on the server side) can never pass.
export function assertIdentityResponse(body, expectedRunToken) {
  if (!body || typeof body !== 'object') {
    throw new Error('e2e identity preflight: identity payload is not an object');
  }
  if (body.database !== 'leanote_test') {
    throw new Error('e2e identity preflight: identity payload does not confirm the leanote_test database');
  }
  if (typeof body.runToken !== 'string' || body.runToken.length === 0) {
    throw new Error('e2e identity preflight: identity payload has no run token');
  }
  if (body.runToken !== expectedRunToken) {
    throw new Error('e2e identity preflight: run token mismatch between harness environment and service');
  }
}

export function requireE2EIdentityConfirmed() {
  if (!confirmed) {
    throw new Error('e2e identity preflight has not completed; call ensureE2EIdentity() before any login, write or route injection');
  }
  return confirmed;
}
