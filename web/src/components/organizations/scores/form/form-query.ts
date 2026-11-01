import type { ScoreForm, ScoreQuery } from '../data';

export const defaultForm: ScoreForm = {
  period: 'last30_days',
  start: '',
  end: '',
  category: 'all',
  search: '',
  repository: '',
  userId: '',
  kinds: '',
  excludeBots: true,
  rules: '',
};

function toLocalInput(iso: string): string {
  const date = new Date(iso);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
}

export function fromQuery(query: ScoreQuery): ScoreForm {
  const { period, filter } = query;
  return {
    period:
      period?.kind === 'custom'
        ? 'custom'
        : period?.kind === 'preset'
          ? (period.preset ?? 'all')
          : 'all',
    start: period?.start ? toLocalInput(period.start) : '',
    end: period?.end ? toLocalInput(period.end) : '',
    category: filter?.preset ?? 'all',
    search: filter?.search ?? '',
    repository: filter?.repositories?.[0] ?? '',
    userId: filter?.users?.[0]?.toString() ?? '',
    kinds: filter?.kinds?.join(', ') ?? '',
    excludeBots: filter?.excludeBots ?? false,
    rules: query.rules ? JSON.stringify(query.rules, null, 2) : '',
  };
}

// Named periods carry no anchor; the server uses the latest ingest.
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
