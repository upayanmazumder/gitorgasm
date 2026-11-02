import type { ComponentProps } from 'react';

import { cn } from '@/lib';

import { controlClassName } from './control';

export function Input({ className, ...props }: ComponentProps<'input'>) {
  return <input className={cn(controlClassName, className)} {...props} />;
}
