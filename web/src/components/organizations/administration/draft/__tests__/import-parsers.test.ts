import { describe, expect, it } from 'vitest';

import { MAX_IMPORT_BYTES } from '../import-result';
import { parseCsv } from '../parse-csv';
import { parseJson } from '../parse-json';
import { parsePasted } from '../parse-pasted';

describe('parsePasted', () => {
  it('splits on spaces, commas, semicolons, and lines and reports each invalid token', () => {
    expect(parsePasted('alice, @bob;carol\ndave  a/b ../x')).toEqual({
      ok: true,
      logins: ['alice', 'bob', 'carol', 'dave'],
      invalid: [
        { position: '', value: 'a/b' },
        { position: '', value: '../x' },
      ],
    });
  });

  it('rejects text over the byte limit before it parses anything', () => {
    expect(parsePasted('a'.repeat(MAX_IMPORT_BYTES + 1))).toEqual({
      ok: false,
      error: 'too_large',
    });
  });

  it('counts bytes, not characters', () => {
    expect(parsePasted('é'.repeat(MAX_IMPORT_BYTES / 2 + 1))).toEqual({
      ok: false,
      error: 'too_large',
    });
  });

  it('reports blank input as empty', () => {
    expect(parsePasted(' \n , ; ')).toEqual({ ok: false, error: 'empty' });
  });
});

describe('parseCsv', () => {
  it('prefers the login column over username and ignores other columns', () => {
    expect(parseCsv('username,name,login\nignored,Alice,alice\nskipped,Bob,bob')).toEqual({
      ok: true,
      logins: ['alice', 'bob'],
      invalid: [],
    });
  });

  it('reads quoted fields with commas, escaped quotes, and line breaks as data', () => {
    const text =
      'login,note\r\n"alice","hello, world"\r\nbob,"say ""hi"""\r\ncarol,"two\nlines"\r\n';
    expect(parseCsv(text)).toEqual({ ok: true, logins: ['alice', 'bob', 'carol'], invalid: [] });
  });

  it('reads a headerless single column including its first row, and skips blank rows', () => {
    expect(parseCsv('alice\n\n  \nbob\n')).toEqual({
      ok: true,
      logins: ['alice', 'bob'],
      invalid: [],
    });
  });

  it('refuses a headerless file with several columns instead of guessing one', () => {
    expect(parseCsv('alice,Alice\nbob,Bob')).toEqual({ ok: false, error: 'no_column' });
  });

  it('reports a quote that never closes with the row where it starts', () => {
    expect(parseCsv('login\nalice\n"bob\ncarol')).toEqual({
      ok: false,
      error: 'unclosed_quote',
      row: 3,
    });
  });

  it('reports every invalid row by its physical row number and keeps the valid ones', () => {
    expect(parseCsv('login\nalice\nbad/name\n\nbob\n,orphan')).toEqual({
      ok: true,
      logins: ['alice', 'bob'],
      invalid: [
        { position: 'Row 3', value: 'bad/name' },
        { position: 'Row 6', value: '' },
      ],
    });
  });

  it('reports a file with only a header as empty', () => {
    expect(parseCsv('login\n')).toEqual({ ok: false, error: 'empty' });
  });
});

describe('parseJson', () => {
  it('reads strings and records with each accepted key', () => {
    const text = JSON.stringify([
      'alice',
      { login: 'bob' },
      { username: 'carol' },
      { github_username: 'dave' },
      { github: '@erin' },
    ]);
    expect(parseJson(text)).toEqual({
      ok: true,
      logins: ['alice', 'bob', 'carol', 'dave', 'erin'],
      invalid: [],
    });
  });

  it('reports items that are not usernames by position and keeps the rest', () => {
    expect(parseJson('["alice", 42, {"name": "x"}, "bad name"]')).toEqual({
      ok: true,
      logins: ['alice'],
      invalid: [
        { position: 'Item 2', value: '42' },
        { position: 'Item 3', value: '{"name":"x"}' },
        { position: 'Item 4', value: 'bad name' },
      ],
    });
  });

  it('rejects an object at the top level and broken syntax', () => {
    expect(parseJson('{"login":"alice"}')).toEqual({ ok: false, error: 'json_shape' });
    expect(parseJson('["alice"')).toEqual({ ok: false, error: 'json_syntax' });
  });

  it('reports an empty array as empty', () => {
    expect(parseJson('[]')).toEqual({ ok: false, error: 'empty' });
  });
});
