import { useState } from 'react';

import { RadioField } from '@/components/ui';

import type { Kind } from '../data';
import type { Actor, Person } from '../draft';
import { ImportPeople } from './import-people';
import { MemberDirectory } from './member-directory';
import { UserSearch } from './user-search';

type Source = 'members' | 'search' | 'paste';

const LABELS: Record<Source, string> = {
  members: 'Current members',
  search: 'GitHub search',
  paste: 'Paste or import',
};

// The members endpoint cannot search, and invites target people outside the organization.
const ALLOWED: Record<Kind, readonly Source[]> = {
  invite: ['search', 'paste'],
  team_membership: ['members', 'search', 'paste'],
  organization_role: ['members', 'paste'],
};

interface Props {
  login: string;
  kind: Kind;
  actor: Actor;
  people: readonly Person[];
  max: number;
  onChange: (people: Person[]) => void;
}

export function SourceField({ login, kind, actor, people, max, onChange }: Props) {
  const [chosen, setChosen] = useState<Source>('members');
  const allowed = ALLOWED[kind];
  const source = allowed.includes(chosen) ? chosen : allowed[0];

  return (
    <div className="space-y-6">
      <fieldset className="min-w-0">
        <legend className="mb-2 text-sm font-medium">Add people from</legend>
        <div className="grid gap-2 md:flex md:flex-wrap md:gap-x-6">
          {allowed.map(option => (
            <RadioField
              key={option}
              name="people-source"
              value={option}
              label={LABELS[option]}
              checked={source === option}
              onChange={() => setChosen(option)}
            />
          ))}
        </div>
      </fieldset>
      {source === 'members' && (
        <MemberDirectory
          login={login}
          kind={kind}
          actor={actor}
          people={people}
          max={max}
          onChange={onChange}
        />
      )}
      {source === 'search' && (
        <UserSearch login={login} people={people} max={max} onChange={onChange} />
      )}
      {source === 'paste' && <ImportPeople people={people} max={max} onChange={onChange} />}
    </div>
  );
}
