import { operationRequestSchema, type Kind, type Role } from '../data';
import { blockedReason } from './people';
import type { Actor, Draft, DraftError, Frozen } from './types';

export const ACTION_LABELS: Record<Kind, string> = {
  invite: 'Invite to the organization',
  team_membership: 'Add to a team',
  organization_role: 'Change organization role',
};

export const ROLE_LABELS: Record<Role, string> = { member: 'Member', admin: 'Organization owner' };

export function people(count: number) {
  return count === 1 ? '1 person' : `${count} people`;
}

export function validateDraft(draft: Draft, actor: Actor): DraftError[] {
  const errors: DraftError[] = [];
  if (draft.people.length === 0) errors.push('no_people');
  if (draft.kind === 'team_membership' && draft.team === null) errors.push('no_team');
  if (draft.kind === 'organization_role' && draft.role === null) errors.push('no_role');
  if (draft.people.some(person => blockedReason(person, draft.kind, actor) !== null)) {
    errors.push('blocked_people');
  }
  return errors;
}

// A parse failure is a programming error, so it throws. validateDraft runs first.
export function buildRequest(draft: Draft): Frozen {
  const logins = draft.people.map(person => person.login);
  const body = operationRequestSchema.parse(
    draft.kind === 'invite'
      ? { kind: 'invite', logins }
      : draft.kind === 'team_membership'
        ? { kind: 'team_membership', logins, team_slug: draft.team?.slug }
        : { kind: 'organization_role', logins, role: draft.role }
  );
  return {
    body,
    teamName: draft.team?.name ?? null,
    unconfirmed: draft.people.filter(person => !person.member).map(person => person.login),
  };
}

export type Acknowledgment = 'owner' | 'outsiders';

// Granting ownership and possible invitations each need an explicit checkbox before any write.
export function requiredAcknowledgment({ body, unconfirmed }: Frozen): Acknowledgment | null {
  if (body.kind === 'organization_role' && body.role === 'admin') return 'owner';
  if (body.kind === 'team_membership' && unconfirmed.length > 0) return 'outsiders';
  return null;
}

// The shape that both a reviewed request and a stored receipt can describe.
export interface Described {
  kind: Kind;
  count: number;
  role: Role | null;
  teamName: string | null;
}

export function describedFrozen({ body, teamName }: Frozen): Described {
  return {
    kind: body.kind,
    count: body.logins.length,
    role: body.kind === 'organization_role' ? body.role : null,
    teamName,
  };
}

export function describeRequest({ kind, count, role, teamName }: Described, org: string) {
  if (kind === 'invite') return `Invite ${people(count)} to ${org} as members.`;
  if (kind === 'team_membership') return `Add ${people(count)} to the ${teamName} team in ${org}.`;
  if (role === 'admin') {
    return count === 1
      ? `Make 1 person an owner of ${org}.`
      : `Make ${count} people owners of ${org}.`;
  }
  return `Set the role of ${people(count)} in ${org} to Member.`;
}

export function confirmLabel({ kind, count, role, teamName }: Described) {
  if (kind === 'invite') return count === 1 ? 'Send 1 invitation' : `Send ${count} invitations`;
  if (kind === 'team_membership') return `Add ${people(count)} to ${teamName}`;
  if (role === 'admin')
    return count === 1 ? 'Make 1 person an owner' : `Make ${count} people owners`;
  return `Set ${people(count)} to Member`;
}
