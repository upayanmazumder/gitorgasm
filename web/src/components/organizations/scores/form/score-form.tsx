'use client';

import type { FormEvent } from 'react';

import { Button, panelVariants } from '@/components/ui';

import { FilterFields, PeriodFields } from './fields';
import type { ScoreFormValues as Form } from './types';

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
    <form onSubmit={submit} className={panelVariants({ className: 'grid gap-4 md:grid-cols-2' })}>
      <PeriodFields form={form} set={set} />
      <FilterFields form={form} set={set} />
      <Button
        type="submit"
        disabled={disabled || pending}
        className="md:col-span-2 md:justify-self-start"
      >
        {pending ? 'Calculating…' : 'Show scores'}
      </Button>
    </form>
  );
}
