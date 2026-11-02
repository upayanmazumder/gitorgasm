import { RadioField } from '@/components/ui';

import type { Kind } from '../data';
import { ACTION_LABELS } from '../draft';

const KINDS: readonly Kind[] = ['invite', 'team_membership', 'organization_role'];

function helpText(kind: Kind, org: string) {
  switch (kind) {
    case 'invite':
      return `GitHub sends each person an invitation to join ${org} as a member.`;
    case 'team_membership':
      return `People join the team as team members. GitHub invites people who are not members of ${org} yet.`;
    case 'organization_role':
      return 'Set the organization role of current members.';
  }
}

interface Props {
  kind: Kind;
  org: string;
  onChange: (kind: Kind) => void;
}

export function ActionField({ kind, org, onChange }: Props) {
  return (
    <fieldset aria-describedby="people-action-help" className="min-w-0 space-y-2">
      <legend className="mb-2 text-sm font-medium">Action</legend>
      <div className="grid gap-2 md:flex md:flex-wrap md:gap-x-6">
        {KINDS.map(option => (
          <RadioField
            key={option}
            name="people-action"
            value={option}
            label={ACTION_LABELS[option]}
            checked={kind === option}
            onChange={() => onChange(option)}
          />
        ))}
      </div>
      <p id="people-action-help" className="max-w-[60ch] text-sm text-muted-foreground">
        {helpText(kind, org)}
      </p>
    </fieldset>
  );
}
