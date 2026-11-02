import { describe, expect, it } from 'vitest';

import { MAX_IMPORT_BYTES } from '../import-result';
import { readImportFile } from '../read-import-file';

function textFile(text: string, name: string) {
  return new File([text], name);
}

// Raw bytes make the encoding explicit, whatever the Blob implementation does with strings.
function byteFile(bytes: number[], name: string) {
  const file = new File(['placeholder'], name);
  file.arrayBuffer = async () => new Uint8Array(bytes).buffer;
  return file;
}

const encode = (text: string) => [...new TextEncoder().encode(text)];

describe('readImportFile', () => {
  it('rejects a file over the limit and accepts one exactly at it', async () => {
    const padding = ' '.repeat(MAX_IMPORT_BYTES - '["alice"]'.length);

    await expect(
      readImportFile(textFile('a'.repeat(MAX_IMPORT_BYTES + 1), 'a.csv'))
    ).resolves.toEqual({
      ok: false,
      error: 'too_large',
    });
    await expect(readImportFile(textFile(`["alice"]${padding}`, 'a.json'))).resolves.toMatchObject({
      ok: true,
      logins: ['alice'],
    });
  });

  it('refuses spreadsheets and other types without reading them', async () => {
    await expect(readImportFile(textFile('alice', 'people.xlsx'))).resolves.toEqual({
      ok: false,
      error: 'wrong_type',
    });
  });

  it('rejects bytes that are not UTF-8', async () => {
    await expect(readImportFile(byteFile([0xc3, 0x28, 0x61], 'people.csv'))).resolves.toEqual({
      ok: false,
      error: 'not_utf8',
    });
  });

  it('ignores a byte order mark so the first header still matches', async () => {
    const bytes = [0xef, 0xbb, 0xbf, ...encode('username\nalice\n')];

    await expect(readImportFile(byteFile(bytes, 'people.csv'))).resolves.toEqual({
      ok: true,
      logins: ['alice'],
      invalid: [],
    });
  });

  it('chooses the parser by file type', async () => {
    await expect(readImportFile(textFile('["alice"]', 'people.JSON'))).resolves.toMatchObject({
      logins: ['alice'],
    });
    await expect(readImportFile(textFile('login\nbob', 'people.CSV'))).resolves.toMatchObject({
      logins: ['bob'],
    });
  });
});
