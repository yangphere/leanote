// Only messages authored by these validators may enter public result artifacts.
export class ReleaseError extends Error {
  constructor(message) {
    super(message);
    this.name = 'ReleaseError';
  }
}

export function describeReleaseError(error) {
  if (error instanceof ReleaseError) return { category: 'contract-rejected', reason: error.message };
  if (error instanceof SyntaxError) return { category: 'invalid-json', reason: 'structured response or artifact is not valid JSON' };
  if (['ENOENT', 'EACCES', 'EPERM'].includes(error?.code)) return { category: error.code, reason: 'required local input is missing or inaccessible' };
  return { category: 'execution-failed', reason: 'operation failed; inspect the named stage using trusted local diagnostics' };
}
