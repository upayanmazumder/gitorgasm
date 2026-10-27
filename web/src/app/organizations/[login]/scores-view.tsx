'use client';

import { useState } from 'react';
import Link from 'next/link';

import { PresetsPanel } from './presets-panel';
import { ScoreForm } from './score-form';
import { defaultForm, errorMessage, fromQuery, toQuery } from './score-query';
import type { Preset } from './score-query';
import { ScoreResults } from './score-results';
import { useScores } from './use-scores';

interface Props {
  login: string;
}

export function ScoresView({ login }: Props) {
  const { daily, presets, calculate, savePreset, removePreset } = useScores(login);
  const [form, setForm] = useState(defaultForm);
  const [presetName, setPresetName] = useState('');
  const [message, setMessage] = useState('');

  function run(action: () => void) {
    setMessage('');
    try {
      action();
    } catch (error) {
      setMessage(errorMessage(error, 'Check the scoring options.'));
    }
  }

  function load(preset: Preset) {
    setPresetName(preset.name);
    setForm(fromQuery(preset.query));
    calculate.reset();
  }

  const save = () =>
    run(() =>
      savePreset.mutate(
        { name: presetName.trim(), query: toQuery(form) },
        {
          onSuccess: () => setMessage('Preset saved.'),
          onError: error => setMessage(errorMessage(error, 'Could not save the preset.')),
        }
      )
    );

  const remove = () =>
    removePreset.mutate(presetName.trim(), {
      onSuccess: () => {
        setPresetName('');
        setMessage('Preset removed.');
      },
      onError: error => setMessage(errorMessage(error, 'Could not remove the preset.')),
    });

  // A custom result from an older data generation is replaced by the newer daily scores.
  const shown =
    calculate.data && calculate.data.state.generation === daily.data?.state.generation
      ? calculate.data
      : daily.data;

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
