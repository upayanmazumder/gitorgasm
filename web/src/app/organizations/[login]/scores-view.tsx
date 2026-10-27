'use client';

import { useState } from 'react';
import type { FormEvent } from 'react';
import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api } from '@/api';

interface ScoreRow {
  userId?: number;
  userLogin?: string;
  repoName?: string;
  score: number;
  counts: { commits: number; pullRequests: number; reviews: number; issues: number };
}

interface ScoreResult {
  data: { contributors: ScoreRow[]; repositories: ScoreRow[]; teams: ScoreRow[] };
  state: { generation: number; last_ingested_at: string | null; ingestion_failed: boolean };
  fresh: boolean;
}

interface ScoreQuery {
  period?: object;
  filter?: object;
  rules?: object;
}

interface Preset {
  name: string;
  query: ScoreQuery;
}

interface Props {
  login: string;
}

function localDate(iso: string) {
  const date = new Date(iso);
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

export function ScoresView({ login }: Props) {
  const base = `/v1/organizations/${encodeURIComponent(login)}/scores`;
  const client = useQueryClient();
  const daily = useQuery({
    queryKey: ['scores', login, base],
    queryFn: async () => (await api.get<ScoreResult>(base)).data,
  });
  const presets = useQuery({
    queryKey: ['score-presets', login, base],
    queryFn: async () => (await api.get<{ data: Preset[] }>(`${base}/presets`)).data.data,
  });
  const calculate = useMutation({
    mutationFn: async (query: ScoreQuery) =>
      (await api.post<ScoreResult>(`${base}/query`, query, { timeout: 120_000 })).data,
  });
  const [period, setPeriod] = useState('last30_days');
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [category, setCategory] = useState('all');
  const [search, setSearch] = useState('');
  const [repository, setRepository] = useState('');
  const [userId, setUserId] = useState('');
  const [kinds, setKinds] = useState('');
  const [excludeBots, setExcludeBots] = useState(true);
  const [rules, setRules] = useState('');
  const [presetName, setPresetName] = useState('');
  const [message, setMessage] = useState('');

  function selectedQuery(): ScoreQuery {
    if (userId && (!Number.isSafeInteger(Number(userId)) || Number(userId) <= 0)) {
      throw new Error('Enter a valid GitHub user ID.');
    }
    const selectedPeriod =
      period === 'all'
        ? { kind: 'all' }
        : period === 'custom'
          ? {
              kind: 'custom',
              start: new Date(start).toISOString(),
              end: new Date(end).toISOString(),
            }
          : { kind: 'preset', preset: period, as_of: daily.data?.state.last_ingested_at };
    return {
      period: selectedPeriod,
      filter: {
        preset: category,
        search: search || null,
        repositories: repository ? [repository] : [],
        users: userId ? [Number(userId)] : [],
        kinds: kinds
          ? kinds
              .split(',')
              .map(value => value.trim())
              .filter(Boolean)
          : [],
        excludeBots,
      },
      ...(rules.trim() ? { rules: JSON.parse(rules) as object } : {}),
    };
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    try {
      setMessage('');
      calculate.mutate(selectedQuery());
    } catch {
      setMessage('Check the dates, user ID, and scoring rules.');
    }
  }

  async function savePreset() {
    if (!presetName.trim()) {
      setMessage('Enter a preset name.');
      return;
    }
    try {
      const query = selectedQuery();
      await api.put(`${base}/presets/${encodeURIComponent(presetName.trim())}`, query);
      await client.invalidateQueries({ queryKey: ['score-presets', login] });
      setMessage('Preset saved.');
    } catch {
      setMessage('Could not save the preset. Check its options.');
    }
  }

  async function removePreset() {
    if (!presetName || !presets.data?.some(preset => preset.name === presetName)) return;
    try {
      await api.delete(`${base}/presets/${encodeURIComponent(presetName)}`);
      await client.invalidateQueries({ queryKey: ['score-presets', login] });
      setPresetName('');
      setMessage('Preset removed.');
    } catch {
      setMessage('Could not remove the preset.');
    }
  }

  function loadPreset(name: string) {
    const query = presets.data?.find(preset => preset.name === name)?.query;
    if (!query) return;
    const savedPeriod = query.period as
      { kind?: string; preset?: string; start?: string; end?: string } | undefined;
    const filter = query.filter as
      | {
          preset?: string;
          search?: string;
          users?: number[];
          repositories?: string[];
          kinds?: string[];
          excludeBots?: boolean;
        }
      | undefined;
    setPresetName(name);
    setPeriod(
      savedPeriod?.kind === 'custom'
        ? 'custom'
        : savedPeriod?.kind === 'preset'
          ? (savedPeriod.preset ?? 'all')
          : 'all'
    );
    setStart(savedPeriod?.start ? localDate(savedPeriod.start) : '');
    setEnd(savedPeriod?.end ? localDate(savedPeriod.end) : '');
    setCategory(filter?.preset ?? 'all');
    setSearch(filter?.search ?? '');
    setRepository(filter?.repositories?.[0] ?? '');
    setUserId(filter?.users?.[0]?.toString() ?? '');
    setKinds(filter?.kinds?.join(', ') ?? '');
    setExcludeBots(filter?.excludeBots ?? false);
    setRules(query.rules ? JSON.stringify(query.rules, null, 2) : '');
    calculate.reset();
  }

  const selected =
    calculate.data?.state.generation === daily.data?.state.generation ? calculate.data : daily.data;
  return (
    <main className="mx-auto max-w-5xl space-y-8 px-6 py-12">
      <Link href="/organizations" className="underline">
        Organizations
      </Link>
      <h1 className="text-3xl font-semibold">{login} scores</h1>
      {daily.isPending && <p role="status">Loading scores…</p>}
      {daily.isError && (
        <p role="alert">Scores are not ready yet. Check again after the next update.</p>
      )}
      {selected && (
        <p role="status">
          {selected.fresh
            ? 'Scores are current.'
            : 'Scores may be old. The latest update is not complete.'}
        </p>
      )}
      <form onSubmit={submit} className="grid gap-4 rounded-lg border p-5 sm:grid-cols-2">
        <label className="grid gap-1">
          Time period
          <select
            value={period}
            onChange={event => setPeriod(event.target.value)}
            className="rounded border p-2"
          >
            <option value="last30_days">Last 30 days</option>
            <option value="last7_days">Last 7 days</option>
            <option value="today">Today</option>
            <option value="this_week">This week</option>
            <option value="this_month">This month</option>
            <option value="all">All available time</option>
            <option value="custom">Choose dates and times</option>
          </select>
        </label>
        <label className="grid gap-1">
          Activity
          <select
            value={category}
            onChange={event => setCategory(event.target.value)}
            className="rounded border p-2"
          >
            <option value="all">All</option>
            <option value="code">Code</option>
            <option value="reviews">Reviews</option>
            <option value="issues">Issues</option>
          </select>
        </label>
        {period === 'custom' && (
          <>
            <label className="grid gap-1">
              Start date and time
              <input
                type="datetime-local"
                required
                value={start}
                onChange={event => setStart(event.target.value)}
                className="rounded border p-2"
              />
            </label>
            <label className="grid gap-1">
              End date and time
              <input
                type="datetime-local"
                required
                value={end}
                onChange={event => setEnd(event.target.value)}
                className="rounded border p-2"
              />
            </label>
          </>
        )}
        <label className="grid gap-1">
          Search people or repositories
          <input
            value={search}
            onChange={event => setSearch(event.target.value)}
            className="rounded border p-2"
          />
        </label>
        <label className="grid gap-1">
          Repository (full name)
          <input
            value={repository}
            onChange={event => setRepository(event.target.value)}
            className="rounded border p-2"
          />
        </label>
        <label className="grid gap-1">
          GitHub user ID
          <input
            type="number"
            min="1"
            value={userId}
            onChange={event => setUserId(event.target.value)}
            className="rounded border p-2"
          />
        </label>
        <label className="grid gap-1">
          Activity types (comma separated)
          <input
            value={kinds}
            onChange={event => setKinds(event.target.value)}
            placeholder="commit, review"
            className="rounded border p-2"
          />
        </label>
        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={excludeBots}
            onChange={event => setExcludeBots(event.target.checked)}
          />{' '}
          Hide bots
        </label>
        <details className="sm:col-span-2">
          <summary>Custom scoring rules</summary>
          <label className="grid gap-1">
            Complete scoring rules in JSON
            <textarea
              value={rules}
              onChange={event => setRules(event.target.value)}
              rows={6}
              className="rounded border p-2 font-mono text-sm"
            />
          </label>
        </details>
        <button
          type="submit"
          disabled={!daily.data || calculate.isPending}
          className="rounded bg-zinc-900 px-4 py-2 text-white disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900"
        >
          {calculate.isPending ? 'Calculating…' : 'Show scores'}
        </button>
      </form>
      <section className="space-y-3" aria-label="Saved presets">
        <h2 className="text-xl font-semibold">Saved presets</h2>
        <label className="grid gap-1">
          Select a preset
          <select
            value={presets.data?.some(preset => preset.name === presetName) ? presetName : ''}
            onChange={event => loadPreset(event.target.value)}
            className="rounded border p-2"
          >
            <option value="">Select saved options</option>
            {presets.data?.map(preset => (
              <option key={preset.name} value={preset.name}>
                {preset.name}
              </option>
            ))}
          </select>
        </label>
        <label className="grid gap-1">
          Preset name
          <input
            value={presetName}
            maxLength={80}
            onChange={event => setPresetName(event.target.value)}
            className="rounded border p-2"
          />
        </label>
        <div className="flex gap-3">
          <button
            type="button"
            onClick={savePreset}
            disabled={!daily.data}
            className="rounded border px-4 py-2"
          >
            Save preset
          </button>
          <button
            type="button"
            onClick={removePreset}
            disabled={!presets.data?.some(preset => preset.name === presetName)}
            className="rounded border px-4 py-2"
          >
            Remove preset
          </button>
        </div>
      </section>
      {message && <p role="status">{message}</p>}
      {calculate.isError && (
        <p role="alert">Could not calculate scores. Check the options or try again.</p>
      )}
      {selected && (
        <section className="space-y-4">
          <h2 className="text-xl font-semibold">Contributors</h2>
          <ol className="space-y-2">
            {selected.data.contributors.map(row => (
              <li key={row.userId} className="flex justify-between rounded border p-3">
                <span>{row.userLogin}</span>
                <strong>{row.score} points</strong>
              </li>
            ))}
          </ol>
          <h2 className="text-xl font-semibold">Repositories</h2>
          <ol className="space-y-2">
            {selected.data.repositories.map(row => (
              <li key={row.repoName} className="flex justify-between rounded border p-3">
                <span>{row.repoName}</span>
                <strong>{row.score} points</strong>
              </li>
            ))}
          </ol>
        </section>
      )}
    </main>
  );
}
