import { isApiError, isServiceUnavailable } from '@/api';

import { REAUTHENTICATION_CODE, type Capability } from '../data';

export type BlockedKind =
  'signed_out' | 'reauthorize' | 'no_access' | NonNullable<Capability['reason']>;

export interface NoticeCopy {
  heading: string;
  body: string[];
}

// Only these three failures prove the capability itself is unavailable. Anything else may be transient.
export function definitiveBlock(error: unknown): BlockedKind | null {
  if (!isApiError(error)) return null;
  if (error.code === REAUTHENTICATION_CODE) return 'reauthorize';
  if (error.status === 401) return 'signed_out';
  if (error.status === 404) return 'no_access';
  return null;
}

export function noticeCopy(kind: BlockedKind, org: string, actorLogin: string): NoticeCopy {
  switch (kind) {
    case 'signed_out':
      return {
        heading: 'Sign in to manage people',
        body: [`Sign in with GitHub to manage people in ${org}.`],
      };
    case 'reauthorize':
      return {
        heading: 'Sign in with GitHub again',
        body: [
          `Your GitHub authorization for Gitorgasm expired or was revoked. Sign in again to manage people in ${org}.`,
        ],
      };
    case 'no_access':
      return {
        heading: 'Organization not available',
        body: [
          `You do not have access to ${org} in Gitorgasm, or the GitHub App is not installed there.`,
        ],
      };
    case 'not_owner':
      return {
        heading: 'Only owners can manage people',
        body: [
          `You are signed in as ${actorLogin}. Only owners of ${org} can invite people, add them to teams, or change roles.`,
        ],
      };
    case 'members_permission_required':
      return {
        heading: 'The GitHub App needs member access',
        body: [
          `Gitorgasm needs the Members read and write permission in ${org}.`,
          'An owner must approve this permission for the Gitorgasm GitHub App on GitHub. Then check again.',
        ],
      };
    case 'installation_suspended':
      return {
        heading: 'The GitHub App is suspended',
        body: [
          `The Gitorgasm GitHub App is suspended for ${org}. An owner must unsuspend it on GitHub before anyone can manage people here.`,
        ],
      };
    case 'unsupported_account':
      return {
        heading: 'People management is not available',
        body: [
          `${org} is not a GitHub organization. Gitorgasm manages people only in organizations.`,
        ],
      };
  }
}

export function accessErrorMessage(error: unknown): string {
  if (isServiceUnavailable(error)) {
    return 'Could not check your access. The service may be temporarily unavailable. Try again in a moment.';
  }
  const code = isApiError(error) ? error.code : undefined;
  if (code === 'github_unavailable' || code === 'github_rate_limited') {
    return 'Could not check your access. GitHub did not answer. Try again in a moment.';
  }
  return 'Could not check your access. Try again.';
}
