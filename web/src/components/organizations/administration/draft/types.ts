import type { Kind, OperationRequest, Rejection, Role } from '../data';

export interface Person {
  // Display casing of the first occurrence, without a leading @.
  login: string;
  // login.toLowerCase(), the deduplication key.
  key: string;
  // GitHub ID when the source supplied it.
  id: number | null;
  // True only when the person came from the current-member directory.
  member: boolean;
}

export interface Team {
  slug: string;
  name: string;
}

export interface Draft {
  kind: Kind;
  people: Person[];
  team: Team | null;
  role: Role | null;
}

// The reviewed request. The body is built once and never rebuilt from the draft.
export interface Frozen {
  body: OperationRequest;
  teamName: string | null;
  // Logins that the directory did not confirm as members, in request order.
  unconfirmed: string[];
}

export type DraftError = 'no_people' | 'no_team' | 'no_role' | 'blocked_people';

export type Phase =
  | { name: 'editing'; errors: DraftError[] }
  | {
      name: 'reviewing';
      request: Frozen;
      acknowledged: boolean;
      ackError: boolean;
      rejection: Rejection | null;
    }
  | { name: 'submitting'; request: Frozen; idempotencyKey: string }
  | { name: 'uncertain'; request: Frozen; idempotencyKey: string; rateLimited: boolean };

export interface WorkspaceState {
  draft: Draft;
  phase: Phase;
}

export interface Actor {
  github_id: number;
  login: string;
}
