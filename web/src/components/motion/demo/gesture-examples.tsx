import { m } from 'framer-motion';

import { panelVariants } from '@/components/ui';
import { spring } from '@/lib';

import { Section } from './section';

export function GestureExamples() {
  return (
    <Section title="Hover & tap" hint="Gesture animations built on the spring presets.">
      <div className="grid grid-cols-2 gap-4">
        <m.div
          whileHover={{ y: -6, scale: 1.03 }}
          transition={spring.snappy}
          className={panelVariants({ className: 'cursor-pointer text-center' })}
        >
          Hover me
        </m.div>
        <m.button
          type="button"
          whileTap={{ scale: 0.95 }}
          transition={spring.snappy}
          className={panelVariants({ className: 'text-center' })}
        >
          Tap me
        </m.button>
      </div>
    </Section>
  );
}
