import type { ComponentProps } from 'react';
import Link from 'next/link';
import type { VariantProps } from 'class-variance-authority';

import { cn } from '@/lib';

import { buttonVariants } from './button';

interface Props extends Omit<ComponentProps<'a'>, 'href'>, VariantProps<typeof buttonVariants> {
  href: string;
  native?: boolean;
}

export function LinkButton({ href, native = false, variant, size, className, ...props }: Props) {
  const classes = cn(buttonVariants({ variant, size }), className);

  return native ? (
    <a href={href} className={classes} {...props} />
  ) : (
    <Link href={href} className={classes} {...props} />
  );
}
