'use client';

import Link from 'next/link';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import * as z from 'zod';

import { isApiError, request } from '@/api';
import { Button, buttonVariants } from '@/components/ui/button';
import { createQueryKeys } from '@/lib/query';

const sessionKeys = createQueryKeys('session');
const organizationKeys = createQueryKeys('organizations');
const userSchema = z.object({
  github_id: z.number(),
  login: z.string(),
  name: z.string().nullable(),
  avatar_url: z.string().nullable(),
});
const organizationsSchema = z.object({
  data: z.array(
    z.object({
      login: z.string(),
      repository_count: z.number(),
      suspended: z.boolean(),
      last_synced_at: z.string().nullable(),
    })
  ),
});

export function OrganizationsView() {
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

  return (
    <main className="mx-auto w-full max-w-3xl space-y-10 px-6 py-12">
      <header className="space-y-4">
        <Link
          href="/"
          className="text-sm underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-4"
        >
          Gitorgasm
        </Link>
        <div className="space-y-2">
          <h1 className="text-3xl font-semibold tracking-tight">Your organizations</h1>
          <p className="max-w-xl text-muted-foreground">
            Connect GitHub to explore activity scores for the organizations you can access.
          </p>
        </div>
      </header>

      {session.isPending ? (
        <p role="status">Checking your session…</p>
      ) : session.isError ? (
        <section className="space-y-4">
          <p role="alert">Could not check your session. Check your connection and try again.</p>
          <Button variant="outline" onClick={() => void session.refetch()}>
            Try again
          </Button>
        </section>
      ) : signedOut ? (
        <section aria-labelledby="sign-in-heading" className="space-y-4 rounded-lg border p-6">
          <h2 id="sign-in-heading" className="text-xl font-semibold">
            Sign in to get started
          </h2>
          <p className="text-muted-foreground">
            Use your GitHub account to see connected organizations. You can install the GitHub App
            after signing in.
          </p>
          <a href="/api/v1/auth/login" className={buttonVariants()}>
            Sign in with GitHub
          </a>
        </section>
      ) : session.data ? (
        <>
          <section aria-labelledby="account-heading" className="space-y-5 border-y py-6">
            <div className="flex flex-wrap items-center justify-between gap-4">
              <div>
                <h2 id="account-heading" className="text-sm text-muted-foreground">
                  Signed in with GitHub
                </h2>
                <p className="font-medium">
                  {session.data.name ? `${session.data.name} ` : null}
                  <span className="text-muted-foreground">@{session.data.login}</span>
                </p>
              </div>
              <Button variant="outline" onClick={() => logout.mutate()} disabled={logout.isPending}>
                {logout.isPending ? 'Signing out…' : 'Sign out'}
              </Button>
            </div>
            <div className="flex flex-wrap gap-3">
              <a href="/api/v1/auth/install" className={buttonVariants()}>
                Install GitHub App
              </a>
              <a href="/api/v1/auth/login" className={buttonVariants({ variant: 'outline' })}>
                Refresh GitHub access
              </a>
            </div>
            <p className="max-w-xl text-sm text-muted-foreground">
              Choose an organization on GitHub to install the app. Already installed it or changed
              your access? Refresh GitHub access to update the organizations available here.
            </p>
            {logout.isPending ? <p role="status">Signing out…</p> : null}
            {logout.isError ? (
              <p role="alert">
                Could not sign out. Your session may still be active. Try signing out again.
              </p>
            ) : null}
          </section>

          <section aria-labelledby="organizations-heading" className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 id="organizations-heading" className="text-xl font-semibold">
                Connected organizations
              </h2>
              <Button
                variant="ghost"
                onClick={() => void organizations.refetch()}
                disabled={organizations.isFetching || logout.isPending}
              >
                {organizations.isFetching ? 'Checking…' : 'Check organizations'}
              </Button>
            </div>
            {organizations.isPending ? (
              <p role="status">Loading organizations…</p>
            ) : organizations.isError ? (
              <p role="alert">
                Could not load your organizations. Check your connection, then choose Check
                organizations to try again.
              </p>
            ) : organizations.data?.length === 0 ? (
              <div className="space-y-2 rounded-lg border border-dashed p-6">
                <h3 className="font-medium">No organizations available yet</h3>
                <p className="text-sm text-muted-foreground">
                  Install the GitHub App for an organization, then complete the GitHub sign-in when
                  you return. If installation needs approval, ask an organization owner to approve
                  it and refresh GitHub access afterward.
                </p>
              </div>
            ) : (
              <nav aria-label="Connected organizations">
                <ul className="divide-y rounded-lg border">
                  {organizations.data?.map(organization => (
                    <li key={organization.login}>
                      <Link
                        href={`/organizations/${encodeURIComponent(organization.login)}`}
                        className="block space-y-1 rounded-lg p-5 hover:bg-muted focus-visible:outline-2 focus-visible:outline-offset-4"
                      >
                        <span className="font-medium">{organization.login}</span>
                        <span className="block text-sm text-muted-foreground">
                          {organization.repository_count}{' '}
                          {organization.repository_count === 1 ? 'repository' : 'repositories'}
                          {organization.suspended
                            ? ' · Installation suspended'
                            : organization.last_synced_at === null
                              ? ' · First sync pending'
                              : null}
                        </span>
                      </Link>
                    </li>
                  ))}
                </ul>
              </nav>
            )}
          </section>
        </>
      ) : null}
    </main>
  );
}
