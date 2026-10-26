import { createEnv } from '@t3-oss/env-nextjs';
import * as z from 'zod';

// Reject paths and trailing slashes that would break the API rewrite.
const originSchema = z
  .url({ protocol: /^https?$/ })
  .refine(value => new URL(value).origin === value, {
    message: 'Must be an origin such as https://api.example.com, without path or trailing slash',
  });

export const env = createEnv({
  server: {
    API_ORIGIN: originSchema,
  },
  experimental__runtimeEnv: {},
  emptyStringAsUndefined: true,
  skipValidation: process.env.SKIP_ENV_VALIDATION === 'true',
});
