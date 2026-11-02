import { useState } from 'react';
import * as z from 'zod';

import { Button, CheckboxField, Field, Input, StatusMessage } from '@/components/ui';
import { useZodForm } from '@/hooks';

import { useUserSearch } from '../data';
import { togglePerson, type Person } from '../draft';
import { ReadError } from '../read-error';
import { LimitNotice } from './limit-notice';

// Mirrors the q parameter of searchAdministrationUsers in contracts/openapi/v1.yaml.
const searchSchema = z.object({
  q: z
    .string()
    .trim()
    .min(1, 'Enter a username or name.')
    .min(2, 'Enter at least 2 characters.')
    .max(64, 'Enter at most 64 characters.')
    .regex(/^[A-Za-z0-9_ -]+$/, 'Use letters, numbers, spaces, hyphens, and underscores only.'),
});

interface Props {
  login: string;
  people: readonly Person[];
  max: number;
  onChange: (people: Person[]) => void;
}

export function UserSearch({ login, people, max, onChange }: Props) {
  const [submitted, setSubmitted] = useState<string | null>(null);
  const form = useZodForm(searchSchema, { defaultValues: { q: '' } });
  const search = useUserSearch(login, submitted);
  const rows = search.data?.pages.flatMap(page => page.data) ?? [];
  const partial = search.data?.pages.some(page => page.incomplete_results) ?? false;
  const error = form.formState.errors.q?.message;

  return (
    <div className="space-y-4">
      <h3>Search public GitHub users</h3>
      <form
        role="search"
        aria-label="Public GitHub users"
        onSubmit={form.handleSubmit(values => setSubmitted(values.q))}
        className="flex flex-col gap-3 md:flex-row md:items-end"
      >
        <Field label="Username or name" className="md:flex-1">
          <Input
            {...form.register('q')}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? 'people-search-error' : 'people-search-help'}
          />
        </Field>
        <Button type="submit" variant="outline">
          Search
        </Button>
      </form>
      {error && (
        <div id="people-search-error">
          <StatusMessage role="alert" tone="error">
            {error}
          </StatusMessage>
        </div>
      )}
      <p id="people-search-help" className="text-sm text-muted-foreground">
        Search finds public accounts only. Paste or import usernames for managed or private
        accounts.
      </p>
      {submitted !== null && search.isPending && (
        <StatusMessage role="status">Searching…</StatusMessage>
      )}
      {search.isSuccess && rows.length === 0 && (
        <StatusMessage role="status">No public GitHub users match “{submitted}”.</StatusMessage>
      )}
      {partial && (
        <StatusMessage role="status">
          GitHub returned partial results. Some matching people may be missing.
        </StatusMessage>
      )}
      {rows.length > 0 && (
        <ul aria-label="Search results" className="divide-y divide-border border-y border-border">
          {rows.map(row => {
            const checked = people.some(person => person.key === row.login.toLowerCase());
            return (
              <li key={row.id} className="py-1">
                <CheckboxField
                  label={row.login}
                  checked={checked}
                  disabled={!checked && people.length >= max}
                  onChange={event =>
                    onChange(
                      togglePerson(
                        people,
                        { login: row.login, id: row.id, member: false },
                        event.target.checked,
                        max
                      )
                    )
                  }
                />
              </li>
            );
          })}
        </ul>
      )}
      {search.isError && (
        <ReadError
          error={search.error}
          subject="search results"
          org={login}
          fetching={search.isFetching}
          onRetry={() =>
            void (search.isFetchNextPageError ? search.fetchNextPage() : search.refetch())
          }
          rateLimited="GitHub search limit reached. Wait a minute, then search again. You can paste usernames instead."
        />
      )}
      {search.hasNextPage && (
        <Button
          variant="outline"
          onClick={() => void search.fetchNextPage()}
          disabled={search.isFetchingNextPage}
        >
          Show more results
        </Button>
      )}
      {people.length >= max && <LimitNotice max={max} />}
    </div>
  );
}
