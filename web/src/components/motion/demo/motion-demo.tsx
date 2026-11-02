'use client';

import { PageShell } from '@/components/layout';

import { DemoHeader } from './demo-header';
import { GestureExamples } from './gesture-examples';
import { PresenceExample } from './presence-example';
import { RevealExamples } from './reveal-examples';

export function MotionDemo() {
  return (
    <PageShell className="space-y-24">
      <DemoHeader />
      <RevealExamples />
      <GestureExamples />
      <PresenceExample />
    </PageShell>
  );
}
