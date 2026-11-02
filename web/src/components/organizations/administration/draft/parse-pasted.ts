import {
  finishImport,
  MAX_IMPORT_BYTES,
  type ImportIssue,
  type ImportResult,
} from './import-result';
import { normalizeLogin } from './login-rule';

export function parsePasted(text: string): ImportResult {
  if (new TextEncoder().encode(text).length > MAX_IMPORT_BYTES) {
    return { ok: false, error: 'too_large' };
  }
  const logins: string[] = [];
  const invalid: ImportIssue[] = [];
  for (const token of text.split(/[\s,;]+/)) {
    if (token === '') continue;
    const login = normalizeLogin(token);
    if (login === null) invalid.push({ position: '', value: token });
    else logins.push(login);
  }
  return finishImport(logins, invalid);
}
