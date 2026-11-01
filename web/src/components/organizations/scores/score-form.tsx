'use client';

import type { FormEvent } from 'react';

import { FilterFields } from './filter-fields';
import { PeriodFields } from './period-fields';
import type { ScoreForm as Form } from './types';

interface Props {
  form: Form;
  onChange: (form: Form) => void;
  onSubmit: () => void;
  disabled: boolean;
  pending: boolean;
}

export function ScoreForm({ form, onChange, onSubmit, disabled, pending }: Props) {
  const set = <K extends keyof Form>(key: K, value: Form[K]) => onChange({ ...form, [key]: value });

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSubmit();
  }

  return (
    <form onSubmit={submit} className="grid gap-4 rounded-lg border p-5 sm:grid-cols-2">
      <PeriodFields form={form} set={set} />
      <FilterFields form={form} set={set} />
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
