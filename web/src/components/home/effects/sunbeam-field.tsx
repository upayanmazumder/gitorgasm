'use client';

import { useEffect, useRef, useState } from 'react';
import dynamic from 'next/dynamic';
import { useTheme } from 'next-themes';

import styles from './effects.module.css';
import { useFieldEffects } from './use-field-effects';

const LightRays = dynamic(
  () => import('@/components/react-bits').then(module => module.LightRays),
  { ssr: false }
);

export function SunbeamField() {
  const fieldRef = useRef<HTMLDivElement>(null);
  const [brand, setBrand] = useState<string | null>(null);
  const enabled = useFieldEffects();
  const { resolvedTheme } = useTheme();
  const light = resolvedTheme !== 'dark';

  useEffect(() => {
    if (!enabled) return;
    // The WebGL vendor needs a literal color, so read the scoped token after theme paint.
    const frame = requestAnimationFrame(() => {
      if (!fieldRef.current) return;
      setBrand(getComputedStyle(fieldRef.current).getPropertyValue('--home-brand').trim());
    });
    return () => cancelAnimationFrame(frame);
  }, [enabled, resolvedTheme]);

  return (
    <div ref={fieldRef} aria-hidden="true" className={styles.field}>
      <div className={styles.beam}>
        {enabled && brand ? (
          <LightRays
            raysOrigin="top-right"
            raysColor={brand}
            raysSpeed={0.08}
            lightSpread={0.7}
            rayLength={1.8}
            intensity={0.6}
            opacity={light ? 0.22 : 0.3}
            followMouse={false}
            cursorShadow={false}
            clickPulse={false}
            intro={false}
            haze={0.12}
            atmosphere={0}
            lightMode={light}
            maxDevicePixelRatio={1}
          />
        ) : null}
      </div>
    </div>
  );
}
