import { MAX_IMPORT_BYTES, type ImportResult } from './import-result';
import { parseCsv } from './parse-csv';
import { parseJson } from './parse-json';

function fileFormat(file: File): 'csv' | 'json' | null {
  const name = file.name.toLowerCase();
  if (name.endsWith('.csv') || file.type === 'text/csv') return 'csv';
  if (name.endsWith('.json') || file.type === 'application/json') return 'json';
  return null;
}

// Spreadsheets are not read. Users export CSV, which avoids a binary parser in the browser.
export async function readImportFile(file: File): Promise<ImportResult> {
  if (file.size > MAX_IMPORT_BYTES) return { ok: false, error: 'too_large' };
  const format = fileFormat(file);
  if (format === null) return { ok: false, error: 'wrong_type' };

  let text: string;
  try {
    text = new TextDecoder('utf-8', { fatal: true }).decode(await file.arrayBuffer());
  } catch {
    return { ok: false, error: 'not_utf8' };
  }
  const withoutBom = text.startsWith('\uFEFF') ? text.slice(1) : text;
  return format === 'csv' ? parseCsv(withoutBom) : parseJson(withoutBom);
}
