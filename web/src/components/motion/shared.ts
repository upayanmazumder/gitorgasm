import type { ReactNode } from 'react';
import type { HTMLMotionProps, Transition } from 'framer-motion';

import { DURATION, EASE } from '@/lib/motion';

export interface MotionWrapperProps extends Omit<
  HTMLMotionProps<'div'>,
  'variants' | 'initial' | 'animate' | 'whileInView'
> {
  children: ReactNode;
  delay?: number;
  duration?: number;
  inView?: boolean;
  once?: boolean;
}

export function resolveTrigger(inView: boolean, once: boolean) {
  return inView
    ? ({ initial: 'hidden', whileInView: 'visible', viewport: { once } } as const)
    : ({ initial: 'hidden', animate: 'visible' } as const);
}

export function resolveTransition(delay = 0, duration: number = DURATION.base): Transition {
  return { duration, delay, ease: EASE.out };
}
