import type { ReactNode } from 'react';

import { cn } from '@/lib';

export function Card({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn('rounded-xl border border-border bg-card p-6', className)}>{children}</div>
  );
}
