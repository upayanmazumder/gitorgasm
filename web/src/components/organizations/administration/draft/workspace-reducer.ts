import type { Kind, Rejection, Role } from '../data';
import type { Draft, DraftError, Frozen, Person, Team, WorkspaceState } from './types';

export type WorkspaceAction =
  | { type: 'setKind'; kind: Kind }
  | { type: 'setPeople'; people: Person[] }
  | { type: 'setTeam'; team: Team | null }
  | { type: 'setRole'; role: Role | null }
  | { type: 'review'; errors: DraftError[]; frozen: Frozen | null }
  | { type: 'edit' }
  | { type: 'acknowledge'; value: boolean }
  | { type: 'ackMissing' }
  | { type: 'submit'; idempotencyKey: string }
  | { type: 'enqueued' }
  | { type: 'rejected'; rejection: Rejection }
  | { type: 'uncertain'; rateLimited: boolean }
  | { type: 'resend' }
  | { type: 'discard' }
  | { type: 'loadRetry'; draft: Draft };

function emptyDraft(kind: Kind): Draft {
  return { kind, people: [], team: null, role: null };
}

export const initialWorkspace: WorkspaceState = {
  draft: emptyDraft('invite'),
  phase: { name: 'editing', errors: [] },
};

function edited(state: WorkspaceState, changes: Partial<Draft>): WorkspaceState {
  if (state.phase.name !== 'editing') return state;
  return { draft: { ...state.draft, ...changes }, phase: { name: 'editing', errors: [] } };
}

// Draft edits are ignored outside editing, so the reviewed payload and its key stay immutable.
export function workspaceReducer(state: WorkspaceState, action: WorkspaceAction): WorkspaceState {
  const { phase } = state;
  switch (action.type) {
    case 'setKind':
      return edited(state, { kind: action.kind });
    case 'setPeople':
      return edited(state, { people: action.people });
    case 'setTeam':
      return edited(state, { team: action.team });
    case 'setRole':
      return edited(state, { role: action.role });
    case 'loadRetry':
      return phase.name === 'editing'
        ? { draft: action.draft, phase: { name: 'editing', errors: [] } }
        : state;
    case 'review':
      if (phase.name !== 'editing') return state;
      if (action.errors.length > 0 || action.frozen === null) {
        return { ...state, phase: { name: 'editing', errors: action.errors } };
      }
      return {
        ...state,
        phase: {
          name: 'reviewing',
          request: action.frozen,
          acknowledged: false,
          ackError: false,
          rejection: null,
        },
      };
    case 'edit':
      return phase.name === 'reviewing'
        ? { ...state, phase: { name: 'editing', errors: [] } }
        : state;
    case 'acknowledge':
      return phase.name === 'reviewing'
        ? { ...state, phase: { ...phase, acknowledged: action.value, ackError: false } }
        : state;
    case 'ackMissing':
      return phase.name === 'reviewing' ? { ...state, phase: { ...phase, ackError: true } } : state;
    case 'submit':
      return phase.name === 'reviewing'
        ? {
            ...state,
            phase: {
              name: 'submitting',
              request: phase.request,
              idempotencyKey: action.idempotencyKey,
            },
          }
        : state;
    case 'enqueued':
      return phase.name === 'submitting'
        ? { draft: emptyDraft(state.draft.kind), phase: { name: 'editing', errors: [] } }
        : state;
    case 'rejected':
      // The server did not accept the request, so its key is dropped.
      return phase.name === 'submitting'
        ? {
            ...state,
            phase: {
              name: 'reviewing',
              request: phase.request,
              acknowledged: false,
              ackError: false,
              rejection: action.rejection,
            },
          }
        : state;
    case 'uncertain':
      return phase.name === 'submitting'
        ? {
            ...state,
            phase: {
              name: 'uncertain',
              request: phase.request,
              idempotencyKey: phase.idempotencyKey,
              rateLimited: action.rateLimited,
            },
          }
        : state;
    case 'resend':
      // Same key and same body object: the server recognizes the repeat.
      return phase.name === 'uncertain'
        ? {
            ...state,
            phase: {
              name: 'submitting',
              request: phase.request,
              idempotencyKey: phase.idempotencyKey,
            },
          }
        : state;
    case 'discard':
      return phase.name === 'uncertain'
        ? { ...state, phase: { name: 'editing', errors: [] } }
        : state;
  }
}
