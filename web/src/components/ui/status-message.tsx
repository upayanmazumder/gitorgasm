import type { ReactNode } from 'react';

import { cn } from '@/lib';

import { StatusIcon, type StatusTone } from './status-icon';

const textTone: Record<StatusTone, string> = {
  neutral: 'text-foreground',
  success: 'text-success',
  error: 'text-destructive',
};

const iconTone: Record<StatusTone, string> = {
  neutral: 'text-muted-foreground',
  success: 'text-success',
  error: 'text-destructive',
};

interface Props {
  role: 'status' | 'alert';
  tone?: StatusTone;
  className?: string;
  children: ReactNode;
}

export function StatusMessage({ role, tone = 'neutral', className, children }: Props) {
  return (
    <p role={role} className={cn('flex items-start gap-2', textTone[tone], className)}>
      <StatusIcon tone={tone} className={cn('mt-0.5', iconTone[tone])} />
      <span>{children}</span>
    </p>
  );
}
