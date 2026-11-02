// Largest accepted pasted text or file, in bytes of UTF-8.
export const MAX_IMPORT_BYTES = 65_536;

export interface ImportIssue {
  // "Row 4", "Item 3", or "" for pasted text.
  position: string;
  value: string;
}

export type ImportError =
  | 'too_large'
  | 'wrong_type'
  | 'not_utf8'
  | 'no_column'
  | 'unclosed_quote'
  | 'json_syntax'
  | 'json_shape'
  | 'empty';

export type ImportResult =
  | { ok: true; logins: string[]; invalid: ImportIssue[] }
  | { ok: false; error: ImportError; row?: number };

export const USERNAME_COLUMNS = ['login', 'username', 'github_username', 'github'] as const;

// Every valid entry is kept and every invalid entry is reported. Nothing disappears.
export function finishImport(logins: string[], invalid: ImportIssue[]): ImportResult {
  if (logins.length === 0 && invalid.length === 0) return { ok: false, error: 'empty' };
  return { ok: true, logins, invalid };
}
