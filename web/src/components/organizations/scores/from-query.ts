import type { ScoreForm, ScoreQuery } from './types';

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
