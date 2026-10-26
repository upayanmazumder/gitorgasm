import type { NextConfig } from 'next';

import { env } from './src/env';

const nextConfig: NextConfig = {
  reactCompiler: true,
  // Same-origin proxy to the API: /api/v1/... -> {API_ORIGIN}/v1/...
  // The API checks Origin on state-changing requests and sets __Host- cookies, both of which
  // require that the browser sees a single origin.
  async rewrites() {
    if (!env.API_ORIGIN) return [];
    return [{ source: '/api/:path*', destination: `${env.API_ORIGIN}/:path*` }];
  },
};

export default nextConfig;
