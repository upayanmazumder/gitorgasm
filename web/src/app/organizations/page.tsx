'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';

import { api } from '@/api';

interface Organization {
  login: string;
  repository_count: number;
}

export default function OrganizationsPage() {
  const organizations = useQuery({
    queryKey: ['organizations'],
    queryFn: async () => (await api.get<{ data: Organization[] }>('/v1/organizations')).data.data,
  });

  return (
    <main className="mx-auto max-w-3xl space-y-6 px-6 py-12">
      <h1 className="text-3xl font-semibold">Your organizations</h1>
      {organizations.isPending && <p role="status">Loading organizations…</p>}
      {organizations.isError && (
        <p role="alert">
          Sign in to see your organizations. <a href="/api/v1/auth/login">Sign in with GitHub</a>
        </p>
      )}
      {organizations.data?.length === 0 && <p>No organizations are connected yet.</p>}
      <ul className="space-y-3">
        {organizations.data?.map(organization => (
          <li key={organization.login} className="rounded-lg border p-4">
            <Link
              href={`/organizations/${encodeURIComponent(organization.login)}`}
              className="font-medium underline"
            >
              {organization.login}
            </Link>
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              {organization.repository_count} repositories
            </p>
          </li>
        ))}
      </ul>
    </main>
  );
}
