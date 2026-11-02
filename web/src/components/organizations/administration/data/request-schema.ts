import * as z from 'zod';

// Mirrors AdministrationLogin in contracts/openapi/v1.yaml. A mismatch makes the whole POST fail.
export const LOGIN_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$/;
// Mirrors the team_slug pattern the API applies after it lowercases the value.
const TEAM_SLUG_PATTERN = /^[a-z0-9][a-z0-9_-]{0,99}$/;

export const roleSchema = z.enum(['member', 'admin']);

const loginsSchema = z.array(z.string().regex(LOGIN_PATTERN)).min(1).max(50);

export const operationRequestSchema = z.discriminatedUnion('kind', [
  z.object({ kind: z.literal('invite'), logins: loginsSchema }).strict(),
  z
    .object({
      kind: z.literal('team_membership'),
      logins: loginsSchema,
      team_slug: z.string().regex(TEAM_SLUG_PATTERN),
    })
    .strict(),
  z
    .object({ kind: z.literal('organization_role'), logins: loginsSchema, role: roleSchema })
    .strict(),
]);

export type OperationRequest = z.infer<typeof operationRequestSchema>;
