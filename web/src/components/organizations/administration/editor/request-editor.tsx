import type { Dispatch } from 'react';

import { Button } from '@/components/ui';

import type { Capability } from '../data';
import { blockedReason, type Draft, type DraftError, type WorkspaceAction } from '../draft';
import { ActionField } from './action-field';
import { DraftErrors } from './draft-errors';
import { RoleField } from './role-field';
import { SelectedPeople } from './selected-people';
import { SourceField } from './source-field';
import { TeamField } from './team-field';

interface Props {
  login: string;
  capability: Capability;
  draft: Draft;
  errors: readonly DraftError[];
  dispatch: Dispatch<WorkspaceAction>;
  onReview: () => void;
  focus: (target: string) => void;
  register: (target: string) => (element: HTMLElement | null) => void;
}

export function RequestEditor({
  login,
  capability,
  draft,
  errors,
  dispatch,
  onReview,
  focus,
  register,
}: Props) {
  const { actor, max_batch_size: max } = capability;
  const setPeople = (people: Draft['people']) => dispatch({ type: 'setPeople', people });
  const blocked = draft.people.filter(person => blockedReason(person, draft.kind, actor) !== null);

  return (
    <div className="space-y-6">
      <ActionField
        kind={draft.kind}
        org={login}
        onChange={kind => dispatch({ type: 'setKind', kind })}
      />
      {draft.kind === 'team_membership' && (
        <TeamField
          login={login}
          team={draft.team}
          onChange={team => dispatch({ type: 'setTeam', team })}
        />
      )}
      {draft.kind === 'organization_role' && (
        <RoleField
          role={draft.role}
          org={login}
          onChange={role => dispatch({ type: 'setRole', role })}
        />
      )}
      <div className="grid gap-8 border-t border-border pt-6 lg:grid-cols-2">
        <SourceField
          login={login}
          kind={draft.kind}
          actor={actor}
          people={draft.people}
          max={max}
          onChange={setPeople}
        />
        <SelectedPeople
          org={login}
          kind={draft.kind}
          actor={actor}
          people={draft.people}
          max={max}
          onChange={setPeople}
          focus={focus}
          register={register}
        />
      </div>
      <DraftErrors
        errors={errors}
        blockedCount={blocked.length}
        onRemoveBlocked={() => {
          setPeople(draft.people.filter(person => !blocked.includes(person)));
          focus('selected');
        }}
        register={register}
      />
      <Button onClick={onReview}>Review request</Button>
    </div>
  );
}
