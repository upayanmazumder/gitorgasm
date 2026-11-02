'use client';

import { m } from 'framer-motion';

import type { SlideDirection } from '@/lib';
import { createSlideVariants } from '@/lib';

import type { MotionWrapperProps } from './shared';
import { resolveTransition, resolveTrigger } from './shared';

interface SlideProps extends MotionWrapperProps {
  direction?: SlideDirection;
  distance?: number;
}

export function Slide({
  children,
  direction = 'up',
  distance = 24,
  delay,
  duration,
  inView = false,
  once = true,
  ...props
}: SlideProps) {
  return (
    <m.div
      variants={createSlideVariants(direction, distance)}
      transition={resolveTransition(delay, duration)}
      {...resolveTrigger(inView, once)}
      {...props}
    >
      {children}
    </m.div>
  );
}
