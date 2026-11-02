import { Button, Field, NativeSelect, StatusMessage } from '@/components/ui';

import { useTeams } from '../data';
import type { Team } from '../draft';
import { ReadError } from '../read-error';

interface Props {
  login: string;
  team: Team | null;
  onChange: (team: Team | null) => void;
}

export function TeamField({ login, team, onChange }: Props) {
  const teams = useTeams(login);
  const items = teams.data?.pages.flatMap(page => page.data) ?? [];

  if (teams.isPending) return <StatusMessage role="status">Loading teams…</StatusMessage>;
  if (teams.isError && items.length === 0) {
    return (
      <ReadError
        error={teams.error}
        subject="teams"
        org={login}
        fetching={teams.isFetching}
        onRetry={() => void teams.refetch()}
      />
    );
  }
  if (items.length === 0) {
    return (
      <div className="space-y-4">
        <StatusMessage role="status">
          {login} has no teams. Create a team on GitHub, then reload the teams.
        </StatusMessage>
        <Button variant="outline" onClick={() => void teams.refetch()} disabled={teams.isFetching}>
          Reload teams
        </Button>
      </div>
    );
  }

  // A retried draft names its team by slug, which can sit on a page that is not loaded yet.
  const selectedLoaded = team === null || items.some(item => item.slug === team.slug);
  return (
    <div className="max-w-md space-y-4">
      <Field label="Team">
        <NativeSelect
          value={team?.slug ?? ''}
          onChange={event => {
            const chosen = items.find(item => item.slug === event.target.value);
            onChange(chosen ? { slug: chosen.slug, name: chosen.name } : null);
          }}
        >
          <option value="">Choose a team</option>
          {!selectedLoaded && team && <option value={team.slug}>{team.name}</option>}
          {items.map(item => (
            <option key={item.id} value={item.slug}>
              {item.name}
            </option>
          ))}
        </NativeSelect>
      </Field>
      {teams.isError && (
        <ReadError
          error={teams.error}
          subject="teams"
          org={login}
          fetching={teams.isFetching}
          onRetry={() =>
            void (teams.isFetchNextPageError ? teams.fetchNextPage() : teams.refetch())
          }
        />
      )}
      {teams.hasNextPage && (
        <Button
          variant="outline"
          onClick={() => void teams.fetchNextPage()}
          disabled={teams.isFetchingNextPage}
        >
          Show more teams
        </Button>
      )}
    </div>
  );
}
