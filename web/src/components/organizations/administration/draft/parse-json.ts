import {
  finishImport,
  USERNAME_COLUMNS,
  type ImportIssue,
  type ImportResult,
} from './import-result';
import { normalizeLogin } from './login-rule';

function usernameOf(item: unknown): unknown {
  if (typeof item !== 'object' || item === null || Array.isArray(item)) return item;
  const record = item as Record<string, unknown>;
  const column = USERNAME_COLUMNS.find(name => typeof record[name] === 'string');
  return column === undefined ? item : record[column];
}

export function parseJson(text: string): ImportResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { ok: false, error: 'json_syntax' };
  }
  if (!Array.isArray(parsed)) return { ok: false, error: 'json_shape' };

  const logins: string[] = [];
  const invalid: ImportIssue[] = [];
  parsed.forEach((item: unknown, index) => {
    const value = usernameOf(item);
    const login = typeof value === 'string' ? normalizeLogin(value) : null;
    if (login !== null) {
      logins.push(login);
      return;
    }
    const shown = typeof value === 'string' ? value : JSON.stringify(item);
    invalid.push({ position: `Item ${index + 1}`, value: shown });
  });
  return finishImport(logins, invalid);
}
