import { useState } from 'react';

import { Button, CheckboxField, Field, NativeSelect, StatusMessage } from '@/components/ui';

import { useMemberDirectory, type Kind, type MemberRole } from '../data';
import { blockedReason, togglePerson, type Actor, type Person } from '../draft';
import { ReadError } from '../read-error';
import { LimitNotice } from './limit-notice';

interface Props {
  login: string;
  kind: Kind;
  actor: Actor;
  people: readonly Person[];
  max: number;
  onChange: (people: Person[]) => void;
}

const FILTERS: ReadonlyArray<{ value: MemberRole; label: string }> = [
  { value: 'all', label: 'All members' },
  { value: 'member', label: 'Members' },
  { value: 'admin', label: 'Owners' },
];

export function MemberDirectory({ login, kind, actor, people, max, onChange }: Props) {
  const [role, setRole] = useState<MemberRole>('all');
  const members = useMemberDirectory(login, role);
  const rows = members.data?.pages.flatMap(page => page.data) ?? [];
  const full = people.length >= max;

  return (
    <div className="space-y-4">
      <h3>Current members</h3>
      <Field label="Show" className="max-w-xs">
        <NativeSelect value={role} onChange={event => setRole(event.target.value as MemberRole)}>
          {FILTERS.map(filter => (
            <option key={filter.value} value={filter.value}>
              {filter.label}
            </option>
          ))}
        </NativeSelect>
      </Field>
      {members.isPending && <StatusMessage role="status">Loading members…</StatusMessage>}
      {!members.isPending && rows.length === 0 && !members.isError && (
        <p className="text-muted-foreground">
          {role === 'all' ? 'No members to show.' : 'No people match this filter.'}
        </p>
      )}
      {rows.length > 0 && (
        <ul aria-label="Current members" className="divide-y divide-border border-y border-border">
          {rows.map(row => {
            const incoming = { login: row.login, id: row.id, member: true };
            const key = row.login.toLowerCase();
            const checked = people.some(person => person.key === key);
            const isSelf = blockedReason({ ...incoming, key }, kind, actor) === 'self';
            const helpId = `people-member-${row.id}-help`;
            return (
              <li key={row.id} className="py-1">
                <CheckboxField
                  label={row.id === actor.github_id ? `${row.login} (you)` : row.login}
                  checked={checked}
                  disabled={isSelf || (!checked && full)}
                  aria-describedby={isSelf ? helpId : undefined}
                  onChange={event =>
                    onChange(togglePerson(people, incoming, event.target.checked, max))
                  }
                />
                {isSelf && (
                  <p id={helpId} className="pl-8 text-sm text-muted-foreground">
                    You cannot change your own role.
                  </p>
                )}
              </li>
            );
          })}
        </ul>
      )}
      {members.isError && (
        <ReadError
          error={members.error}
          subject="members"
          org={login}
          fetching={members.isFetching}
          onRetry={() =>
            void (members.isFetchNextPageError ? members.fetchNextPage() : members.refetch())
          }
        />
      )}
      {members.hasNextPage && (
        <Button
          variant="outline"
          onClick={() => void members.fetchNextPage()}
          disabled={members.isFetchingNextPage}
        >
          Show more members
        </Button>
      )}
      {full && <LimitNotice max={max} />}
    </div>
  );
}
