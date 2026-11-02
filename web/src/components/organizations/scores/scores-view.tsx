'use client';

import { PageHeader, PageShell } from '@/components/layout';
import { Button, LinkButton, StatusMessage } from '@/components/ui';

import { ScoreForm } from './form';
import { PresetsPanel } from './presets';
import { ScoreResults } from './results';
import { errorMessage, useScoreControls } from './state';

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
    submit,
    load,
    save,
    remove,
    shown,
  } = useScoreControls(login);

  return (
    <PageShell width="wide">
      <PageHeader
        lead={
          <LinkButton href="/organizations" variant="link">
            Organizations
          </LinkButton>
        }
        title={`${login} scores`}
      />
      {daily.isPending && <StatusMessage role="status">Loading scores…</StatusMessage>}
      {daily.isError && (
        <div className="space-y-4">
          <StatusMessage role="alert" tone="error">
            {errorMessage(daily.error, 'Could not load scores. Try again.')}
          </StatusMessage>
          <Button
            variant="outline"
            onClick={() => void daily.refetch()}
            disabled={daily.isFetching}
          >
            Try again
          </Button>
        </div>
      )}
      <ScoreForm
        form={form}
        onChange={setForm}
        onSubmit={submit}
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
      {message && <StatusMessage role="status">{message}</StatusMessage>}
      {calculate.isError && (
        <StatusMessage role="alert" tone="error">
          {errorMessage(calculate.error, 'Could not calculate scores.')}
        </StatusMessage>
      )}
      {shown && <ScoreResults result={shown} />}
    </PageShell>
  );
}
