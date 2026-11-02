import {
  finishImport,
  USERNAME_COLUMNS,
  type ImportIssue,
  type ImportResult,
} from './import-result';
import { normalizeLogin } from './login-rule';

interface CsvRecord {
  row: number;
  cells: string[];
}

// RFC 4180 subset: comma fields, CRLF/LF/CR records, quoted fields with "" escapes and embedded breaks.
function readRecords(text: string): { records: CsvRecord[]; unclosedRow: number | null } {
  const records: CsvRecord[] = [];
  let cells: string[] = [];
  let field = '';
  let quoted = false;
  let row = 1;
  let quoteRow = 1;

  const endField = () => {
    cells.push(field.trim());
    field = '';
  };
  const endRecord = () => {
    endField();
    records.push({ row, cells });
    cells = [];
    row += 1;
  };

  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (quoted) {
      if (char !== '"') field += char;
      else if (text[index + 1] === '"') {
        field += '"';
        index += 1;
      } else quoted = false;
    } else if (char === '"' && field.trim() === '') {
      field = '';
      quoted = true;
      quoteRow = row;
    } else if (char === ',') endField();
    else if (char === '\n' || char === '\r') {
      if (char === '\r' && text[index + 1] === '\n') index += 1;
      endRecord();
    } else field += char;
  }
  if (quoted) return { records, unclosedRow: quoteRow };
  if (field !== '' || cells.length > 0) endRecord();
  return { records, unclosedRow: null };
}

function headerColumn(first: CsvRecord): number {
  const names = first.cells.map(cell => cell.toLowerCase());
  for (const name of USERNAME_COLUMNS) {
    const index = names.indexOf(name);
    if (index !== -1) return index;
  }
  return -1;
}

export function parseCsv(text: string): ImportResult {
  const { records, unclosedRow } = readRecords(text);
  if (unclosedRow !== null) return { ok: false, error: 'unclosed_quote', row: unclosedRow };

  // A record whose cells are all empty carries no data.
  const filled = records.filter(record => record.cells.some(cell => cell !== ''));
  const column = filled.length > 0 ? headerColumn(filled[0]) : -1;
  const hasHeader = column !== -1;
  const data = hasHeader ? filled.slice(1) : filled;
  const singleColumn = filled.every(
    record => record.cells.filter(cell => cell !== '').length === 1
  );
  if (!hasHeader && !singleColumn) return { ok: false, error: 'no_column' };

  const logins: string[] = [];
  const invalid: ImportIssue[] = [];
  for (const record of data) {
    const value = hasHeader
      ? (record.cells[column] ?? '')
      : (record.cells.find(cell => cell !== '') ?? '');
    const login = normalizeLogin(value);
    if (login !== null) logins.push(login);
    else invalid.push({ position: `Row ${record.row}`, value });
  }
  return finishImport(logins, invalid);
}
