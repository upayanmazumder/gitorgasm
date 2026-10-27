import { isApiError } from '@/api';

export interface ScoreRow {
  userId?: number;
  userLogin?: string;
  repoName?: string;
  score: number;
  rank: number;
}

export interface ScoreResult {
  data: { contributors: ScoreRow[]; repositories: ScoreRow[]; teams: ScoreRow[] };
  state: { generation: number; last_ingested_at: string | null; ingestion_failed: boolean };
  fresh: boolean;
}

export interface ScoreQuery {
  period?: { kind: string; preset?: string; start?: string; end?: string };
  filter?: {
    preset?: string;
    search?: string | null;
    users?: number[];
    repositories?: string[];
    kinds?: string[];
    excludeBots?: boolean;
  };
  rules?: object;
}

export interface Preset {
  name: string;
  query: ScoreQuery;
}

export interface ScoreForm {
  period: string;
  start: string;
  end: string;
  category: string;
  search: string;
  repository: string;
  userId: string;
  kinds: string;
  excludeBots: boolean;
  rules: string;
}

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

/** Maps API error codes to a sentence the user can act on. */
export function errorMessage(error: unknown, fallback: string): string {
  if (!isApiError(error)) return error instanceof Error ? error.message : fallback;
  switch (error.code) {
    case 'invalid_query':
      return 'The scoring options are not valid. Check the dates, filters, and rules.';
    case 'invalid_name':
      return 'Preset names need 1 to 80 printable characters.';
    case 'preset_limit':
      return 'This organization has the maximum number of presets. Remove one first.';
    case 'scores_pending':
      return 'Scores are not ready yet. Check again after the next update.';
    case 'scores_busy':
      return 'Scoring is busy. Try again in a moment.';
    case 'organization_not_found':
      return 'You do not have access to this organization.';
  }
  if (error.status === 401) return 'Your session ended. Sign in again.';
  return fallback;
}
