'use client';

import { DemoHeader } from './demo-header';
import { GestureExamples } from './gesture-examples';
import { PresenceExample } from './presence-example';
import { RevealExamples } from './reveal-examples';

export function MotionDemo() {
  return (
    <div className="mx-auto w-full max-w-3xl space-y-24 px-6 py-24">
      <DemoHeader />
      <RevealExamples />
      <GestureExamples />
      <PresenceExample />
    </div>
  );
}
