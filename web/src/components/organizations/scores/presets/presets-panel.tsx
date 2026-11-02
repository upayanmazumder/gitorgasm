'use client';

import { Button, Field, Input, NativeSelect, panelVariants } from '@/components/ui';

import type { Preset } from '../data';

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
    <section
      className={panelVariants({ className: 'space-y-4' })}
      aria-labelledby="presets-heading"
    >
      <h2 id="presets-heading">Saved presets</h2>
      <Field label="Select a preset">
        <NativeSelect
          value={saved ? name.trim() : ''}
          onChange={event => {
            const preset = presets.find(p => p.name === event.target.value);
            if (preset) onLoad(preset);
          }}
        >
          <option value="">Select saved options</option>
          {presets.map(preset => (
            <option key={preset.name} value={preset.name}>
              {preset.name}
            </option>
          ))}
        </NativeSelect>
      </Field>
      <Field label="Preset name">
        <Input value={name} maxLength={80} onChange={event => onNameChange(event.target.value)} />
      </Field>
      <div className="flex flex-wrap gap-3">
        <Button type="button" variant="outline" onClick={onSave} disabled={busy || !name.trim()}>
          Save preset
        </Button>
        <Button type="button" variant="outline" onClick={onRemove} disabled={busy || !saved}>
          Remove preset
        </Button>
      </div>
    </section>
  );
}
