import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api } from '@/api';

import type { Preset, ScoreQuery, ScoreResult } from './types';

// The API stops a custom calculation after 25 seconds; leave room for the response.
const CALCULATE_TIMEOUT_MS = 30_000;

export function useScores(login: string) {
  const base = `/v1/organizations/${encodeURIComponent(login)}/scores`;
  const client = useQueryClient();
  const presetsKey = ['score-presets', base];

  const daily = useQuery({
    queryKey: ['scores', base],
    queryFn: async () => (await api.get<ScoreResult>(base)).data,
    retry: false,
  });
  const presets = useQuery({
    queryKey: ['score-presets', base],
    queryFn: async () => (await api.get<{ data: Preset[] }>(`${base}/presets`)).data.data,
  });
  const calculate = useMutation({
    mutationFn: async (query: ScoreQuery) =>
      (await api.post<ScoreResult>(`${base}/query`, query, { timeout: CALCULATE_TIMEOUT_MS })).data,
  });
  const savePreset = useMutation({
    mutationFn: async ({ name, query }: { name: string; query: ScoreQuery }) =>
      (await api.put<Preset>(`${base}/presets/${encodeURIComponent(name)}`, query)).data,
    onSuccess: () => client.invalidateQueries({ queryKey: presetsKey }),
  });
  const removePreset = useMutation({
    mutationFn: async (name: string) => api.delete(`${base}/presets/${encodeURIComponent(name)}`),
    onSuccess: () => client.invalidateQueries({ queryKey: presetsKey }),
  });

  return { daily, presets, calculate, savePreset, removePreset };
}
