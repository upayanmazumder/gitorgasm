import { createEnv } from '@t3-oss/env-nextjs';
import * as z from 'zod';

export const env = createEnv({
  server: {
    // Origin of the Gitorgasm API. `next.config.ts` proxies `/api/*` to it, so the browser only
    // talks to this app's origin and the session cookie stays first-party.
    API_ORIGIN: z.url(),
  },
  experimental__runtimeEnv: {},
  emptyStringAsUndefined: true,
  skipValidation: !!process.env.SKIP_ENV_VALIDATION,
});
