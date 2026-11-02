import { useQuery } from '@tanstack/react-query';

import { isServiceUnavailable } from '@/api';

import { adminGet } from './admin-get';
import { memberKeys, operationDetailKey, operationKeys } from './keys';
import { isActive } from './operation-status';
import { operationSchema, type Operation } from './schemas';

// Each detail read makes the API check owner state on GitHub, so 3 seconds keeps a 50-person run near 30 to 60 calls.
const POLL_INTERVAL_MS = 3_000;
// Matches the operation id pattern in contracts/openapi/v1.yaml.
const RECEIPT_ID = /^[A-Za-z0-9_-]{43}$/;

export function isReceiptId(value: string | null): value is string {
  return value !== null && RECEIPT_ID.test(value);
}

function shouldPoll({ data, error }: { data: Operation | undefined; error: unknown }) {
  if (!data || !isActive(data)) return false;
  return error === null || isServiceUnavailable(error);
}

export function useOperation(login: string, id: string | null) {
  return useQuery({
    queryKey: operationDetailKey(login, id ?? ''),
    queryFn: async context => {
      const previous = context.client.getQueryData<Operation>(context.queryKey);
      const { data } = await adminGet(context, login, `/operations/${id}`, operationSchema);
      // A terminal first read has no previous value. A history list that loaded earlier can still hold a
      // nonterminal summary, so that read also refreshes the list.
      if (!isActive(data) && (!previous || isActive(previous))) {
        // History is mounted beside the receipt, so it must refetch now. Closing the receipt removes the
        // live status override and would expose the stale row.
        void context.client.invalidateQueries({ queryKey: operationKeys.list({ login }) });
        // Member lists are not mounted here. They refetch on their next mount.
        void context.client.invalidateQueries({
          queryKey: memberKeys.lists(),
          refetchType: 'none',
        });
      }
      return data;
    },
    enabled: isReceiptId(id),
    retry: false,
    staleTime: query => (query.state.data && !isActive(query.state.data) ? Infinity : 0),
    refetchOnWindowFocus: true,
    refetchInterval: query => (shouldPoll(query.state) ? POLL_INTERVAL_MS : false),
    refetchIntervalInBackground: false,
  });
}
