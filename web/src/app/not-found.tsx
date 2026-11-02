import type { Metadata } from 'next';

import { NotFoundView } from '@/components/not-found';

export const metadata: Metadata = {
  title: 'Page not found',
};

export default function NotFound() {
  return <NotFoundView />;
}
