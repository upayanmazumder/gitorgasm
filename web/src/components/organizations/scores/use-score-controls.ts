import { useState } from 'react';

import { defaultForm } from './default-form';
import { errorMessage } from './error-message';
import { fromQuery } from './from-query';
import { toQuery } from './to-query';
import type { Preset } from './types';
import { useScores } from './use-scores';

export function useScoreControls(login: string) {
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

  return {
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
  };
}
