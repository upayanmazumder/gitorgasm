import { useInfiniteQuery } from '@tanstack/react-query';

import { adminGet } from './admin-get';
import { userKeys } from './keys';
import { usersPageSchema } from './schemas';

// GitHub's unauthenticated search quota is low, so a repeat search of the same text reuses the cache.
export function useUserSearch(login: string, q: string | null) {
  return useInfiniteQuery({
    queryKey: userKeys.list({ login, q }),
    queryFn: context =>
      adminGet(context, login, '/users', usersPageSchema, { q, cursor: context.pageParam }),
    initialPageParam: null as string | null,
    getNextPageParam: last => last.next_cursor ?? undefined,
    enabled: q !== null,
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
  });
}
