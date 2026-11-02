import type { Kind, Operation, ResultStatus, TargetResult } from '../data';

// Display order: what needs attention first, then what succeeded.
const GROUP_ORDER: readonly ResultStatus[] = [
  'running',
  'unknown',
  'failed',
  'not_attempted',
  'invited',
  'succeeded',
  'unchanged',
  'queued',
];

const SUMMARY_ORDER: readonly ResultStatus[] = [
  'succeeded',
  'invited',
  'unchanged',
  'failed',
  'unknown',
  'not_attempted',
];

const LABELS: Record<ResultStatus, Record<Kind, string>> = {
  succeeded: {
    invite: 'Invitation sent',
    team_membership: 'Added to team',
    organization_role: 'Role changed',
  },
  invited: {
    invite: 'Invitation sent',
    team_membership: 'Invited to organization and team',
    organization_role: 'Invitation sent',
  },
  unchanged: {
    invite: 'Already a member or invited',
    team_membership: 'Already on the team or invited',
    organization_role: 'Already has this role',
  },
  failed: { invite: 'Failed', team_membership: 'Failed', organization_role: 'Failed' },
  unknown: {
    invite: 'Outcome unknown',
    team_membership: 'Outcome unknown',
    organization_role: 'Outcome unknown',
  },
  not_attempted: {
    invite: 'Not attempted',
    team_membership: 'Not attempted',
    organization_role: 'Not attempted',
  },
  running: {
    invite: 'In progress',
    team_membership: 'In progress',
    organization_role: 'In progress',
  },
  queued: { invite: 'Waiting', team_membership: 'Waiting', organization_role: 'Waiting' },
};

const SUMMARY_TERMS: Partial<Record<ResultStatus, string>> = {
  succeeded: 'changed',
  invited: 'invited',
  unchanged: 'unchanged',
  failed: 'failed',
  unknown: 'unknown',
  not_attempted: 'not attempted',
};

// An invitation can report succeeded or invited. Both mean the same to the user.
function groupOf(kind: Kind, status: ResultStatus): ResultStatus {
  return kind === 'invite' && status === 'succeeded' ? 'invited' : status;
}

export interface ResultGroup {
  status: ResultStatus;
  label: string;
  results: TargetResult[];
}

export function groupResults({
  kind,
  results,
}: Pick<Operation, 'kind' | 'results'>): ResultGroup[] {
  return GROUP_ORDER.map(status => ({
    status,
    label: LABELS[status][kind],
    results: results.filter(result => groupOf(kind, result.status) === status),
  })).filter(group => group.results.length > 0);
}

export function countStatus(operation: Operation, status: ResultStatus) {
  return operation.results.filter(result => groupOf(operation.kind, result.status) === status)
    .length;
}

export function summarize(operation: Operation) {
  return SUMMARY_ORDER.map(status => ({ status, count: countStatus(operation, status) }))
    .filter(term => term.count > 0)
    .map(term => `${term.count} ${SUMMARY_TERMS[term.status]}`)
    .join(', ');
}

export function hasProblems(operation: Operation) {
  return (
    countStatus(operation, 'failed') +
      countStatus(operation, 'unknown') +
      countStatus(operation, 'not_attempted') >
    0
  );
}

export function countDone(operation: Operation) {
  return operation.results.filter(
    result => result.status !== 'queued' && result.status !== 'running'
  ).length;
}
