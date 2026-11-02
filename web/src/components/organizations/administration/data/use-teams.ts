import { useInfiniteQuery } from '@tanstack/react-query';

import { adminGet } from './admin-get';
import { teamKeys } from './keys';
import { teamsPageSchema } from './schemas';

export function useTeams(login: string) {
  return useInfiniteQuery({
    queryKey: teamKeys.list({ login }),
    queryFn: context =>
      adminGet(context, login, '/teams', teamsPageSchema, { cursor: context.pageParam }),
    initialPageParam: null as string | null,
    getNextPageParam: last => last.next_cursor ?? undefined,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
