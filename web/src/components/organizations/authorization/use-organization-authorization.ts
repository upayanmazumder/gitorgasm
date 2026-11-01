import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { isApiError, request } from '@/api';
import { createQueryKeys } from '@/lib/query';

import { organizationsSchema, userSchema } from './schemas';

const sessionKeys = createQueryKeys('session');
const organizationKeys = createQueryKeys('organizations');

export function useOrganizationAuthorization() {
  const client = useQueryClient();
  const session = useQuery({
    queryKey: sessionKeys.all(),
    queryFn: async ({ signal }) => {
      try {
        return await request({ url: '/v1/me', signal, schema: userSchema });
      } catch (error) {
        if (isApiError(error) && error.status === 401) return null;
        throw error;
      }
    },
    retry: false,
    staleTime: 0,
    refetchOnWindowFocus: true,
  });
  const organizations = useQuery({
    queryKey: organizationKeys.list({ user: session.data?.github_id }),
    queryFn: async ({ signal }) =>
      (await request({ url: '/v1/organizations', signal, schema: organizationsSchema })).data,
    enabled: !!session.data,
    retry: false,
    staleTime: 0,
    refetchOnWindowFocus: true,
  });
  const logout = useMutation({
    mutationFn: () => request({ method: 'POST', url: '/v1/auth/logout' }),
    onSuccess: async () => {
      await client.cancelQueries();
      // Preserve the active session observer while removing every tenant's cached data.
      client.removeQueries({ predicate: query => query.queryKey[0] !== 'session' });
      client.getMutationCache().clear();
      client.setQueryData(sessionKeys.all(), null);
    },
  });
  const expired = isApiError(organizations.error) && organizations.error.status === 401;
  const signedOut = session.data === null || expired;

  return { session, organizations, logout, signedOut };
}
