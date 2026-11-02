import type { Operation } from '../data';
import type { Draft } from './types';

// Only not_attempted people are safe to retry. unknown needs a check on GitHub, and failed will likely fail again.
export function retryDraft(operation: Operation): Draft {
  return {
    kind: operation.kind,
    people: operation.results
      .filter(result => result.status === 'not_attempted')
      .map(result => ({
        login: result.login,
        key: result.login.toLowerCase(),
        id: null,
        member: false,
      })),
    team: operation.team_slug ? { slug: operation.team_slug, name: operation.team_slug } : null,
    role: operation.role,
  };
}
