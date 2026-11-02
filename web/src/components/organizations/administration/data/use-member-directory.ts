import { useInfiniteQuery } from '@tanstack/react-query';

import { adminGet } from './admin-get';
import { memberKeys } from './keys';
import { membersPageSchema } from './schemas';

export type MemberRole = 'all' | 'member' | 'admin';

// Each page loads only on request, so a large organization never fetches every member.
export function useMemberDirectory(login: string, role: MemberRole) {
  return useInfiniteQuery({
    queryKey: memberKeys.list({ login, role }),
    queryFn: context =>
      adminGet(context, login, '/members', membersPageSchema, {
        role,
        cursor: context.pageParam,
      }),
    initialPageParam: null as string | null,
    getNextPageParam: last => last.next_cursor ?? undefined,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
