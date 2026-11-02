import { useInfiniteQuery } from '@tanstack/react-query';

import { adminGet } from './admin-get';
import { operationKeys } from './keys';
import { isActive } from './operation-status';
import { operationsPageSchema } from './schemas';

// The list never polls. The active receipt shows its live status from its own detail query. A window
// focus refreshes the list only while a row still reads as active, because closing a running receipt
// removes the only live status.
export function useRecentOperations(login: string) {
  return useInfiniteQuery({
    queryKey: operationKeys.list({ login }),
    queryFn: context =>
      adminGet(context, login, '/operations', operationsPageSchema, { cursor: context.pageParam }),
    initialPageParam: null as string | null,
    getNextPageParam: last => last.next_cursor ?? undefined,
    staleTime: 0,
    refetchOnWindowFocus: query =>
      query.state.data?.pages.some(page => page.data.some(isActive)) ?? false,
    retry: false,
  });
}
