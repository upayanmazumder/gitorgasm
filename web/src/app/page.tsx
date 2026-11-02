import type { Metadata } from 'next';

import { HomeView } from '@/components/home';

export const metadata: Metadata = {
  description: 'Activity scores for the contributors and repositories in your GitHub organization.',
};

export default function Home() {
  return <HomeView />;
}
