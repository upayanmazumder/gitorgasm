import { LOGIN_PATTERN } from '../data';

// Accepts managed-user underscores. Rejects slashes, dots, spaces, and URLs, so no value can alter an API path.
export function normalizeLogin(raw: string): string | null {
  const trimmed = raw.trim();
  const login = trimmed.startsWith('@') ? trimmed.slice(1) : trimmed;
  return LOGIN_PATTERN.test(login) ? login : null;
}
