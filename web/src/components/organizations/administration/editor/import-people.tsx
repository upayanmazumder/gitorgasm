import { useState, type ChangeEvent } from 'react';
import * as z from 'zod';

import { Button, Field, Input, StatusMessage, Textarea } from '@/components/ui';
import { useZodForm } from '@/hooks';

import {
  MAX_IMPORT_BYTES,
  mergePeople,
  parsePasted,
  readImportFile,
  type ImportResult,
  type Person,
} from '../draft';
import { ImportReport, type ImportOutcome, type ImportSource } from './import-report';

const pasteSchema = z.object({
  text: z
    .string()
    .refine(
      text => new TextEncoder().encode(text).length <= MAX_IMPORT_BYTES,
      'Paste at most 64 KiB of text.'
    ),
});

interface Props {
  people: readonly Person[];
  max: number;
  onChange: (people: Person[]) => void;
}

export function ImportPeople({ people, max, onChange }: Props) {
  const [report, setReport] = useState<{ source: ImportSource; outcome: ImportOutcome } | null>(
    null
  );
  const form = useZodForm(pasteSchema, { defaultValues: { text: '' } });
  const error = form.formState.errors.text?.message;

  // Returns true when the people were added, so the caller can clear what it consumed.
  function apply(source: ImportSource, result: ImportResult) {
    if (!result.ok) {
      setReport({ source, outcome: { error: result.error, row: result.row } });
      return false;
    }
    const incoming = result.logins.map(login => ({ login, id: null, member: false }));
    const merged = mergePeople(people, incoming, max);
    if (!merged.overflow) onChange(merged.people);
    setReport({ source, outcome: { ...merged, invalid: result.invalid } });
    return !merged.overflow;
  }

  function addPasted({ text }: { text: string }) {
    const result = parsePasted(text);
    // Only invalid tokens stay, so the user can fix them in place.
    if (apply({ kind: 'paste' }, result) && result.ok) {
      form.reset({ text: result.invalid.map(issue => issue.value).join('\n') });
    }
  }

  async function addFile(event: ChangeEvent<HTMLInputElement>) {
    const input = event.currentTarget;
    const file = input.files?.[0];
    if (!file) return;
    apply({ kind: 'file', name: file.name }, await readImportFile(file));
    // Clearing the input lets the same file be chosen again.
    input.value = '';
  }

  return (
    <div className="space-y-4">
      <h3>Paste or import usernames</h3>
      <form onSubmit={form.handleSubmit(addPasted)} className="space-y-4">
        <Field label="Usernames">
          <Textarea
            rows={5}
            {...form.register('text')}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? 'people-paste-error' : 'people-paste-help'}
          />
        </Field>
        {error && (
          <div id="people-paste-error">
            <StatusMessage role="alert" tone="error">
              {error}
            </StatusMessage>
          </div>
        )}
        <p id="people-paste-help" className="text-sm text-muted-foreground">
          Separate usernames with spaces, commas, or new lines. A leading @ is allowed.
        </p>
        <Button type="submit" variant="outline">
          Add usernames
        </Button>
      </form>
      <div className="space-y-2 border-t border-border pt-6">
        <Field label="CSV or JSON file">
          <Input
            type="file"
            accept=".csv,.json,text/csv,application/json"
            aria-describedby="people-file-help"
            onChange={event => void addFile(event)}
          />
        </Field>
        <p id="people-file-help" className="text-sm text-muted-foreground">
          Up to 64 KiB of UTF-8 text. A CSV file needs a login, username, github_username, or github
          column, or one username per row. A JSON file needs an array of usernames, or of objects
          with one of those fields.
        </p>
      </div>
      {report && <ImportReport source={report.source} outcome={report.outcome} max={max} />}
    </div>
  );
}
