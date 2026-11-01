import type { ScoreForm, ScoreQuery } from './types';

/** Builds the API query. Named periods carry no anchor; the server uses the latest ingest. */
export function toQuery(form: ScoreForm): ScoreQuery {
  const userId = form.userId.trim();
  if (userId && !(Number.isSafeInteger(Number(userId)) && Number(userId) > 0)) {
    throw new Error('Enter a valid GitHub user ID.');
  }
  let period: ScoreQuery['period'];
  if (form.period === 'all') {
    period = { kind: 'all' };
  } else if (form.period === 'custom') {
    const start = new Date(form.start);
    const end = new Date(form.end);
    if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || start >= end) {
      throw new Error('Choose a start time before the end time.');
    }
    period = { kind: 'custom', start: start.toISOString(), end: end.toISOString() };
  } else {
    period = { kind: 'preset', preset: form.period };
  }
  let rules: object | undefined;
  if (form.rules.trim()) {
    try {
      rules = JSON.parse(form.rules) as object;
    } catch {
      throw new Error('Scoring rules must be valid JSON.');
    }
  }
  return {
    period,
    filter: {
      preset: form.category,
      search: form.search.trim() || null,
      repositories: form.repository.trim() ? [form.repository.trim()] : [],
      users: userId ? [Number(userId)] : [],
      kinds: form.kinds
        .split(',')
        .map(value => value.trim())
        .filter(Boolean),
      excludeBots: form.excludeBots,
    },
    ...(rules ? { rules } : {}),
  };
}
