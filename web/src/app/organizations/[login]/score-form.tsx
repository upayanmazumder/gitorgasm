'use client';

import type { FormEvent } from 'react';

import type { ScoreForm as Form } from './score-query';

interface Props {
  form: Form;
  onChange: (form: Form) => void;
  onSubmit: () => void;
  disabled: boolean;
  pending: boolean;
}

const input = 'rounded border p-2';

export function ScoreForm({ form, onChange, onSubmit, disabled, pending }: Props) {
  const set = <K extends keyof Form>(key: K, value: Form[K]) => onChange({ ...form, [key]: value });
  const text = (key: 'start' | 'end' | 'search' | 'repository' | 'userId' | 'kinds') => ({
    value: form[key],
    onChange: (event: { target: { value: string } }) => set(key, event.target.value),
    className: input,
  });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit();
  }

  return (
    <form onSubmit={submit} className="grid gap-4 rounded-lg border p-5 sm:grid-cols-2">
      <label className="grid gap-1">
        Time period (UTC)
        <select value={form.period} onChange={e => set('period', e.target.value)} className={input}>
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
          value={form.category}
          onChange={e => set('category', e.target.value)}
          className={input}
        >
          <option value="all">All</option>
          <option value="code">Code</option>
          <option value="reviews">Reviews</option>
          <option value="issues">Issues</option>
        </select>
      </label>
      {form.period === 'custom' && (
        <>
          <label className="grid gap-1">
            Start date and time
            <input type="datetime-local" required {...text('start')} />
          </label>
          <label className="grid gap-1">
            End date and time
            <input type="datetime-local" required {...text('end')} />
          </label>
        </>
      )}
      <label className="grid gap-1">
        Search people or repositories
        <input {...text('search')} />
      </label>
      <label className="grid gap-1">
        Repository (full name)
        <input {...text('repository')} />
      </label>
      <label className="grid gap-1">
        GitHub user ID
        <input type="number" min="1" {...text('userId')} />
      </label>
      <label className="grid gap-1">
        Activity types (comma separated)
        <input placeholder="commit, review" {...text('kinds')} />
      </label>
      <label className="flex items-center gap-2">
        <input
          type="checkbox"
          checked={form.excludeBots}
          onChange={e => set('excludeBots', e.target.checked)}
        />
        Hide bots
      </label>
      <details className="sm:col-span-2">
        <summary>Custom scoring rules</summary>
        <label className="grid gap-1">
          Complete scoring rules in JSON
          <textarea
            value={form.rules}
            onChange={e => set('rules', e.target.value)}
            rows={6}
            className={`${input} font-mono text-sm`}
          />
        </label>
      </details>
      <button
        type="submit"
        disabled={disabled || pending}
        className="rounded bg-zinc-900 px-4 py-2 text-white disabled:opacity-50 dark:bg-zinc-100 dark:text-zinc-900"
      >
        {pending ? 'Calculating…' : 'Show scores'}
      </button>
    </form>
  );
}
