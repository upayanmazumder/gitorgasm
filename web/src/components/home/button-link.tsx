import type { ReactNode } from 'react';
import Link from 'next/link';

import styles from './button-link.module.css';

interface Props {
  href: string;
  variant: 'primary' | 'secondary' | 'text';
  compact?: boolean;
  className?: string;
  children: ReactNode;
}

export function ButtonLink({ href, variant, compact = false, className = '', children }: Props) {
  const classes = [styles[variant], compact ? styles.compact : '', className];

  return (
    <Link href={href} className={classes.filter(Boolean).join(' ')}>
      {children}
    </Link>
  );
}
