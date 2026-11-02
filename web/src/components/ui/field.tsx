import type { ReactNode } from 'react';

import { cn } from '@/lib';

interface Props {
  label: string;
  className?: string;
  children: ReactNode;
}

export function Field({ label, className, children }: Props) {
  return (
    <label className={cn('grid gap-2', className)}>
      <span className="text-sm font-medium">{label}</span>
      {children}
    </label>
  );
}
