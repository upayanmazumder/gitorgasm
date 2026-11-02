import type { ComponentProps } from 'react';

import { cn } from '@/lib';

import { controlClassName } from './control';

export function NativeSelect({ className, children, ...props }: ComponentProps<'select'>) {
  return (
    <span className="relative block">
      <select className={cn(controlClassName, 'appearance-none pr-10', className)} {...props}>
        {children}
      </select>
      <svg
        viewBox="0 0 20 20"
        width="20"
        height="20"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
        focusable="false"
        className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2"
      >
        <path d="m6 8 4 4 4-4" />
      </svg>
    </span>
  );
}
