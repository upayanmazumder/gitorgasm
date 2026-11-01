import { m } from 'framer-motion';

import { spring } from '@/lib/motion';

import { Section } from './section';

export function GestureExamples() {
  return (
    <Section title="Hover & tap" hint="Gesture animations built on the spring presets.">
      <div className="grid grid-cols-2 gap-4">
        <m.div
          whileHover={{ y: -6, scale: 1.03 }}
          transition={spring.snappy}
          className="cursor-pointer rounded-xl border border-border bg-card p-6 text-center"
        >
          Hover me
        </m.div>
        <m.button
          type="button"
          whileTap={{ scale: 0.95 }}
          transition={spring.snappy}
          className="rounded-xl border border-border bg-card p-6 text-center"
        >
          Tap me
        </m.button>
      </div>
    </Section>
  );
}
