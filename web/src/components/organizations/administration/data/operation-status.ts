import type { Operation } from './schemas';

export function isActive(operation: Pick<Operation, 'status'>) {
  return operation.status === 'queued' || operation.status === 'running';
}
