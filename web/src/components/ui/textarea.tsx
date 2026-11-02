import type { ComponentProps } from 'react';

import { cn } from '@/lib';

import { controlClassName } from './control';

export function Textarea({ className, ...props }: ComponentProps<'textarea'>) {
  return <textarea className={cn(controlClassName, className)} {...props} />;
}
