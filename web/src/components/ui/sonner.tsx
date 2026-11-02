'use client';

import type { ComponentProps, CSSProperties } from 'react';
import { useTheme } from 'next-themes';
import { Toaster as SonnerToaster } from 'sonner';

type ToasterProps = ComponentProps<typeof SonnerToaster>;

const style = {
  '--normal-bg': 'var(--card)',
  '--normal-text': 'var(--foreground)',
  '--normal-border': 'var(--border-strong)',
  '--border-radius': 'var(--control-radius)',
  fontFamily: 'var(--typeface-text)',
} as CSSProperties;

export function Toaster(props: ToasterProps) {
  const { theme = 'system' } = useTheme();
  return (
    <SonnerToaster
      theme={theme as ToasterProps['theme']}
      className="toaster group"
      style={style}
      {...props}
    />
  );
}
