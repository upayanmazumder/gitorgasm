import type { Kind } from '../data';
import type { Actor, Person } from './types';

export interface IncomingPerson {
  login: string;
  id: number | null;
  member: boolean;
}

export interface MergeResult {
  people: Person[];
  added: number;
  duplicates: number;
  total: number;
  overflow: boolean;
}

export function mergePeople(
  existing: readonly Person[],
  incoming: readonly IncomingPerson[],
  max: number
): MergeResult {
  const next = [...existing];
  const positions = new Map(next.map((person, index) => [person.key, index]));
  let added = 0;
  let duplicates = 0;

  for (const candidate of incoming) {
    const key = candidate.login.toLowerCase();
    const position = positions.get(key);
    if (position === undefined) {
      positions.set(key, next.length);
      next.push({ login: candidate.login, key, id: candidate.id, member: candidate.member });
      added += 1;
      continue;
    }
    duplicates += 1;
    const current = next[position];
    // A directory hit proves membership and identity. The first position and casing stay.
    next[position] = {
      ...current,
      id: current.id ?? candidate.id,
      member: current.member || candidate.member,
    };
  }

  if (next.length > max) {
    return { people: [...existing], added: 0, duplicates, total: next.length, overflow: true };
  }
  return { people: next, added, duplicates, total: next.length, overflow: false };
}

// Checking merges the person like any other source. Unchecking removes by key.
export function togglePerson(
  people: readonly Person[],
  incoming: IncomingPerson,
  checked: boolean,
  max: number
): Person[] {
  if (checked) return mergePeople(people, [incoming], max).people;
  const key = incoming.login.toLowerCase();
  return people.filter(person => person.key !== key);
}

export type BlockedReason = 'member' | 'self';

export function blockedReason(person: Person, kind: Kind, actor: Actor): BlockedReason | null {
  if (kind === 'invite' && person.member) return 'member';
  if (
    kind === 'organization_role' &&
    (person.id === actor.github_id || person.key === actor.login.toLowerCase())
  ) {
    return 'self';
  }
  return null;
}

export function blockedNote(reason: BlockedReason, org: string) {
  return reason === 'member'
    ? `Cannot invite: already a member of ${org}.`
    : 'Cannot include: this is you. You cannot change your own role.';
}

export function personNote(person: Person, kind: Kind, org: string): string | null {
  if (person.member) return 'Current member';
  if (kind === 'team_membership') {
    return `Not confirmed as a member. GitHub may invite this person to ${org}.`;
  }
  if (kind === 'organization_role') {
    return 'Membership not checked. Gitorgasm changes roles for active members only.';
  }
  return null;
}
