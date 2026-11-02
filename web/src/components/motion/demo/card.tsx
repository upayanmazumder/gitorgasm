import type { ReactNode } from 'react';

import { panelVariants } from '@/components/ui';
import { cn } from '@/lib';

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn(panelVariants(), className)}>{children}</div>;
}
