import { describe, expect, it } from 'vitest';
import * as z from 'zod';

import { cursorPageSchema, cursorQuerySchema, dateTimeSchema } from '../common';

describe('dateTimeSchema', () => {
  it('accepts RFC 3339 timestamps in UTC and with an offset', () => {
    expect(dateTimeSchema.safeParse('2026-10-26T08:00:00Z').success).toBe(true);
    expect(dateTimeSchema.safeParse('2026-10-26T08:00:00.123+05:30').success).toBe(true);
  });

  it('rejects a date without a time', () => {
    expect(dateTimeSchema.safeParse('2026-10-26').success).toBe(false);
  });
});

describe('cursorQuerySchema', () => {
  it('accepts limits inside the contract bounds', () => {
    expect(cursorQuerySchema.safeParse({ limit: 1 }).success).toBe(true);
    expect(cursorQuerySchema.safeParse({ limit: 100, cursor: 'abc' }).success).toBe(true);
  });

  it('rejects limits outside the contract bounds', () => {
    expect(cursorQuerySchema.safeParse({ limit: 0 }).success).toBe(false);
    expect(cursorQuerySchema.safeParse({ limit: 101 }).success).toBe(false);
  });

  it('rejects an empty cursor', () => {
    expect(cursorQuerySchema.safeParse({ cursor: '' }).success).toBe(false);
  });
});

describe('cursorPageSchema', () => {
  const schema = cursorPageSchema(z.object({ login: z.string() }));

  it('parses a middle page and the last page', () => {
    expect(schema.parse({ data: [{ login: 'a' }], next_cursor: 'c1' }).next_cursor).toBe('c1');
    expect(schema.parse({ data: [], next_cursor: null }).next_cursor).toBeNull();
  });

  it('requires next_cursor to be present', () => {
    expect(schema.safeParse({ data: [] }).success).toBe(false);
  });

  it('rejects items that do not match the item schema', () => {
    expect(schema.safeParse({ data: [{ login: 1 }], next_cursor: null }).success).toBe(false);
  });
});
