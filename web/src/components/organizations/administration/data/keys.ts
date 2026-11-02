import { createQueryKeys } from '@/lib';

export const accessKeys = createQueryKeys('administration');
export const memberKeys = createQueryKeys('administration-members');
export const teamKeys = createQueryKeys('administration-teams');
export const userKeys = createQueryKeys('administration-users');
export const operationKeys = createQueryKeys('administration-operations');

// The id alone could collide across organizations, so the detail key carries both.
export function operationDetailKey(login: string, id: string) {
  return [...operationKeys.details(), login, id] as const;
}
