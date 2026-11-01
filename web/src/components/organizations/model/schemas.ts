import * as z from 'zod';

export const userSchema = z.object({
  github_id: z.number(),
  login: z.string(),
  name: z.string().nullable(),
  avatar_url: z.string().nullable(),
});

export const organizationsSchema = z.object({
  data: z.array(
    z.object({
      login: z.string(),
      repository_count: z.number(),
      suspended: z.boolean(),
      last_synced_at: z.string().nullable(),
    })
  ),
});

export type OrganizationUser = z.infer<typeof userSchema>;
export type Organization = z.infer<typeof organizationsSchema>['data'][number];
