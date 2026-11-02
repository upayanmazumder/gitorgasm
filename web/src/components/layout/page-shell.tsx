import type { ReactNode } from 'react';

import { cn } from '@/lib';

interface Props {
  width?: 'narrow' | 'wide';
  className?: string;
  children: ReactNode;
}

export function PageShell({ width = 'narrow', className, children }: Props) {
  return (
    <main
      className={cn(
        'mx-auto w-full space-y-12 px-5 py-12 md:px-8 md:py-16 lg:px-12',
        width === 'wide' ? 'max-w-5xl' : 'max-w-3xl',
        className
      )}
    >
      {children}
    </main>
  );
}
