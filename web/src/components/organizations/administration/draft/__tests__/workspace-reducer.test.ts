import { describe, expect, it } from 'vitest';

import type { Frozen, Person, WorkspaceState } from '../types';
import { initialWorkspace, workspaceReducer, type WorkspaceAction } from '../workspace-reducer';

const alice: Person = { login: 'alice', key: 'alice', id: null, member: false };
const frozen: Frozen = {
  body: { kind: 'invite', logins: ['alice'] },
  teamName: null,
  unconfirmed: ['alice'],
};

function run(actions: WorkspaceAction[], from: WorkspaceState = initialWorkspace) {
  return actions.reduce(workspaceReducer, from);
}

const reviewing = run([
  { type: 'setPeople', people: [alice] },
  { type: 'review', errors: [], frozen },
]);

describe('workspaceReducer', () => {
  it('ignores draft edits once a request is under review, in flight, or uncertain', () => {
    const edits: WorkspaceAction[] = [
      { type: 'setKind', kind: 'organization_role' },
      { type: 'setPeople', people: [] },
      { type: 'setTeam', team: { slug: 'x', name: 'X' } },
      { type: 'setRole', role: 'admin' },
    ];
    const inFlight = run([{ type: 'submit', idempotencyKey: 'k1' }], reviewing);
    const uncertain = run([{ type: 'uncertain', rateLimited: false }], inFlight);

    for (const state of [reviewing, inFlight, uncertain]) {
      expect(run(edits, state)).toBe(state);
    }
  });

  it('stays in editing with errors instead of reviewing an invalid draft', () => {
    const state = run([{ type: 'review', errors: ['no_people'], frozen: null }]);

    expect(state.phase).toEqual({ name: 'editing', errors: ['no_people'] });
  });

  it('drops the acknowledgment when the user edits and reviews again', () => {
    const acknowledged = run([{ type: 'acknowledge', value: true }], reviewing);
    const again = run([{ type: 'edit' }, { type: 'review', errors: [], frozen }], acknowledged);

    expect(acknowledged.phase).toMatchObject({ acknowledged: true });
    expect(again.phase).toMatchObject({ name: 'reviewing', acknowledged: false });
  });

  it('keeps the same key and the same body object when an uncertain request is resent', () => {
    const uncertain = run(
      [
        { type: 'submit', idempotencyKey: 'k1' },
        { type: 'uncertain', rateLimited: true },
      ],
      reviewing
    );
    const resent = run([{ type: 'resend' }], uncertain);

    expect(resent.phase).toMatchObject({ name: 'submitting', idempotencyKey: 'k1' });
    expect(resent.phase.name === 'submitting' && resent.phase.request.body).toBe(frozen.body);
  });

  it('drops the key when the server rejects the request, so a later confirm needs a new one', () => {
    const rejected = run(
      [
        { type: 'submit', idempotencyKey: 'k1' },
        { type: 'rejected', rejection: 'invalid' },
      ],
      reviewing
    );

    expect(rejected.phase).toEqual({
      name: 'reviewing',
      request: frozen,
      acknowledged: false,
      ackError: false,
      rejection: 'invalid',
    });
  });

  it('keeps the draft when an uncertain request is discarded and clears it after an accepted one', () => {
    const uncertain = run(
      [
        { type: 'submit', idempotencyKey: 'k1' },
        { type: 'uncertain', rateLimited: false },
        { type: 'discard' },
      ],
      reviewing
    );
    const accepted = run(
      [{ type: 'submit', idempotencyKey: 'k2' }, { type: 'enqueued' }],
      reviewing
    );

    expect(uncertain.draft.people).toEqual([alice]);
    expect(accepted.draft.people).toEqual([]);
    expect(accepted.phase).toEqual({ name: 'editing', errors: [] });
  });

  it('only loads a retry draft while editing', () => {
    const retry = { kind: 'invite' as const, people: [alice], team: null, role: null };

    expect(run([{ type: 'loadRetry', draft: retry }]).draft).toEqual(retry);
    expect(run([{ type: 'loadRetry', draft: retry }], reviewing)).toBe(reviewing);
  });
});
