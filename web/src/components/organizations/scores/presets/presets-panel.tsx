'use client';

import type { Preset } from '../data';
import { inputClassName } from '../form/fields';

interface Props {
  presets: Preset[];
  name: string;
  onNameChange: (name: string) => void;
  onLoad: (preset: Preset) => void;
  onSave: () => void;
  onRemove: () => void;
  busy: boolean;
}

export function PresetsPanel({
  presets,
  name,
  onNameChange,
  onLoad,
  onSave,
  onRemove,
  busy,
}: Props) {
  const saved = presets.some(preset => preset.name === name.trim());

  return (
    <section className="space-y-3" aria-labelledby="presets-heading">
      <h2 id="presets-heading" className="text-xl font-semibold">
        Saved presets
      </h2>
      <label className="grid gap-1">
        Select a preset
        <select
          value={saved ? name.trim() : ''}
          onChange={event => {
            const preset = presets.find(p => p.name === event.target.value);
            if (preset) onLoad(preset);
          }}
          className={inputClassName}
        >
          <option value="">Select saved options</option>
          {presets.map(preset => (
            <option key={preset.name} value={preset.name}>
              {preset.name}
            </option>
          ))}
        </select>
      </label>
      <label className="grid gap-1">
        Preset name
        <input
          value={name}
          maxLength={80}
          onChange={event => onNameChange(event.target.value)}
          className={inputClassName}
        />
      </label>
      <div className="flex gap-3">
        <button
          type="button"
          onClick={onSave}
          disabled={busy || !name.trim()}
          className="rounded border px-4 py-2 disabled:opacity-50"
        >
          Save preset
        </button>
        <button
          type="button"
          onClick={onRemove}
          disabled={busy || !saved}
          className="rounded border px-4 py-2 disabled:opacity-50"
        >
          Remove preset
        </button>
      </div>
    </section>
  );
}
