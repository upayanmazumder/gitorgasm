import { RadioField } from '@/components/ui';

import type { Role } from '../data';
import { ROLE_LABELS } from '../draft';

const ROLES: readonly Role[] = ['member', 'admin'];

interface Props {
  role: Role | null;
  org: string;
  onChange: (role: Role) => void;
}

export function RoleField({ role, org, onChange }: Props) {
  const help: Record<Role, string> = {
    member: `A regular member of ${org}. Owners in this request lose owner access.`,
    admin: `Complete administrative access to ${org}.`,
  };
  return (
    <fieldset className="min-w-0 space-y-2">
      <legend className="mb-2 text-sm font-medium">New role</legend>
      <div className="grid gap-2 md:flex md:flex-wrap md:gap-x-6">
        {ROLES.map(option => (
          <div key={option}>
            <RadioField
              name="people-role"
              value={option}
              label={ROLE_LABELS[option]}
              checked={role === option}
              onChange={() => onChange(option)}
              aria-describedby={`people-role-help-${option}`}
            />
            <p id={`people-role-help-${option}`} className="pl-8 text-sm text-muted-foreground">
              {help[option]}
            </p>
          </div>
        ))}
      </div>
    </fieldset>
  );
}
