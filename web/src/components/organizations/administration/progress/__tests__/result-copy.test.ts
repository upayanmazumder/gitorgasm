import { describe, expect, it } from 'vitest';

import type { Operation, ResultStatus } from '../../data';
import { groupResults, hasProblems, summarize } from '../result-copy';

function operation(kind: Operation['kind'], statuses: ResultStatus[]): Operation {
  return {
    id: 'op',
    kind,
    status: 'completed',
    team_slug: null,
    role: null,
    target_count: statuses.length,
    created_at: '2026-11-03T10:00:00Z',
    completed_at: null,
    results: statuses.map((status, index) => ({
      login: `user${index}`,
      status,
      code: null,
      message: null,
    })),
  };
}

describe('result summary', () => {
  it('counts an invitation as invited whether it reports succeeded or invited', () => {
    const result = operation('invite', ['succeeded', 'invited', 'unchanged']);

    expect(summarize(result)).toBe('2 invited, 1 unchanged');
    expect(groupResults(result).map(group => group.label)).toEqual([
      'Invitation sent',
      'Already a member or invited',
    ]);
  });

  it('keeps changed and invited apart for other actions', () => {
    expect(summarize(operation('team_membership', ['succeeded', 'invited']))).toBe(
      '1 changed, 1 invited'
    );
  });

  it('lists what needs attention before what succeeded and never hides a problem', () => {
    const result = operation('organization_role', [
      'succeeded',
      'not_attempted',
      'unknown',
      'failed',
    ]);

    expect(groupResults(result).map(group => group.status)).toEqual([
      'unknown',
      'failed',
      'not_attempted',
      'succeeded',
    ]);
    expect(summarize(result)).toBe('1 changed, 1 failed, 1 unknown, 1 not attempted');
    expect(hasProblems(result)).toBe(true);
    expect(hasProblems(operation('organization_role', ['succeeded', 'unchanged']))).toBe(false);
  });
});
