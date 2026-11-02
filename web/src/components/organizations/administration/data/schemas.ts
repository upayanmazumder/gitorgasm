import * as z from 'zod';

import { roleSchema } from './request-schema';

const kindSchema = z.enum(['invite', 'team_membership', 'organization_role']);
const identitySchema = z.object({ id: z.number(), login: z.string() });

export const accessSchema = z.object({
  data: z.object({
    organization: z.string(),
    actor: z.object({ github_id: z.number(), login: z.string() }),
    can_manage: z.boolean(),
    reason: z
      .enum([
        'not_owner',
        'members_permission_required',
        'unsupported_account',
        'installation_suspended',
      ])
      .nullable(),
    max_batch_size: z.number().int().positive(),
  }),
});

export const usersPageSchema = z.object({
  data: z.array(identitySchema),
  next_cursor: z.string().nullable(),
  incomplete_results: z.boolean(),
});

export const membersPageSchema = z.object({
  data: z.array(identitySchema),
  next_cursor: z.string().nullable(),
});

export const teamsPageSchema = z.object({
  data: z.array(z.object({ id: z.number(), name: z.string(), slug: z.string() })),
  next_cursor: z.string().nullable(),
});

const summarySchema = z.object({
  id: z.string(),
  kind: kindSchema,
  status: z.enum(['queued', 'running', 'completed', 'interrupted']),
  team_slug: z.string().nullable(),
  role: roleSchema.nullable(),
  target_count: z.number().int(),
  created_at: z.string(),
  completed_at: z.string().nullable(),
});

const resultSchema = z.object({
  login: z.string(),
  status: z.enum([
    'queued',
    'running',
    'succeeded',
    'invited',
    'unchanged',
    'failed',
    'unknown',
    'not_attempted',
  ]),
  code: z.string().nullable(),
  message: z.string().nullable(),
});

export const operationSchema = z.object({
  data: summarySchema.extend({ results: z.array(resultSchema) }),
});

export const operationsPageSchema = z.object({
  data: z.array(summarySchema),
  next_cursor: z.string().nullable(),
});

export type Kind = z.infer<typeof kindSchema>;
export type Role = z.infer<typeof roleSchema>;
export type Capability = z.infer<typeof accessSchema>['data'];
export type Operation = z.infer<typeof operationSchema>['data'];
export type OperationSummary = z.infer<typeof summarySchema>;
export type TargetResult = z.infer<typeof resultSchema>;
export type ResultStatus = TargetResult['status'];
