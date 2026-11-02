'use client';

import { lazy, Suspense } from 'react';
import type { CSSProperties, ReactNode } from 'react';

import styles from './effects.module.css';
import { useFieldEffects } from './use-field-effects';

const SpotlightCard = lazy(() =>
  import('@/components/react-bits').then(module => ({ default: module.SpotlightCard }))
);

// The vendor reads these variables, so the panel keeps the homepage surface and border tokens.
const palette = {
  '--spotlight-card-surface': 'var(--home-surface)',
  '--spotlight-card-border': 'var(--home-line)',
  '--spotlight-card-shadow': 'none',
} as CSSProperties;

interface Props {
  children: ReactNode;
  className?: string;
}

export function SpotlightPanel({ children, className = '' }: Props) {
  const enabled = useFieldEffects();
  const surface = `${styles.surface} ${className}`;

  return enabled ? (
    <Suspense fallback={<div className={surface}>{children}</div>}>
      <SpotlightCard
        className={surface}
        style={palette}
        spotlightColor="var(--home-brand)"
        intensity={0.06}
        borderGlow={0.2}
        proximity={0}
        flare={false}
        ambient={false}
      >
        {children}
      </SpotlightCard>
    </Suspense>
  ) : (
    <div className={surface}>{children}</div>
  );
}
