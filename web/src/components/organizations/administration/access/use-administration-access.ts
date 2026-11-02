import { useQuery } from '@tanstack/react-query';

import { request } from '@/api';

import { accessKeys, accessSchema, administrationPath } from '../data';

// A capability decision never outlives its page view. The API checks owner state again before every write.
export function useAdministrationAccess(login: string) {
  return useQuery({
    queryKey: accessKeys.detail(login),
    queryFn: async ({ signal }) =>
      (await request({ url: administrationPath(login), signal, schema: accessSchema })).data,
    retry: false,
    staleTime: 0,
    refetchOnWindowFocus: true,
  });
}
