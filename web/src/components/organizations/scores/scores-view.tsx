'use client';

import Link from 'next/link';

import { errorMessage } from './error-message';
import { PresetsPanel } from './presets-panel';
import { ScoreForm } from './score-form';
import { ScoreResults } from './score-results';
import { toQuery } from './to-query';
import { useScoreControls } from './use-score-controls';

interface Props {
  login: string;
}

export function ScoresView({ login }: Props) {
  const {
    daily,
    presets,
    calculate,
    savePreset,
    removePreset,
    form,
    setForm,
    presetName,
    setPresetName,
    message,
    run,
    load,
    save,
    remove,
    shown,
  } = useScoreControls(login);

  return (
    <main className="mx-auto max-w-5xl space-y-8 px-6 py-12">
      <Link href="/organizations" className="underline">
        Organizations
      </Link>
      <h1 className="text-3xl font-semibold">{login} scores</h1>
      {daily.isPending && <p role="status">Loading scores…</p>}
      {daily.isError && (
        <p role="alert">{errorMessage(daily.error, 'Could not load scores. Try again later.')}</p>
      )}
      <ScoreForm
        form={form}
        onChange={setForm}
        onSubmit={() => run(() => calculate.mutate(toQuery(form)))}
        disabled={!daily.data}
        pending={calculate.isPending}
      />
      <PresetsPanel
        presets={presets.data ?? []}
        name={presetName}
        onNameChange={setPresetName}
        onLoad={load}
        onSave={save}
        onRemove={remove}
        busy={savePreset.isPending || removePreset.isPending}
      />
      {message && <p role="status">{message}</p>}
      {calculate.isError && (
        <p role="alert">{errorMessage(calculate.error, 'Could not calculate scores.')}</p>
      )}
      {shown && <ScoreResults result={shown} />}
    </main>
  );
}
