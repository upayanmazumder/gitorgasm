'use client';

import { useState } from 'react';
import { AnimatePresence, m } from 'framer-motion';

import { Button } from '@/components/ui';
import { spring } from '@/lib/motion';

import { Card } from './card';
import { Section } from './section';

export function PresenceExample() {
  const [open, setOpen] = useState(false);

  return (
    <Section title="AnimatePresence" hint="Animate an element as it mounts and unmounts.">
      <div className="space-y-4">
        <Button variant="outline" onClick={() => setOpen(o => !o)}>
          {open ? 'Unmount' : 'Mount'}
        </Button>
        <AnimatePresence initial={false}>
          {open && (
            <m.div
              key="panel"
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={spring.soft}
              className="overflow-hidden"
            >
              <Card>This panel animates both its mount and its unmount.</Card>
            </m.div>
          )}
        </AnimatePresence>
      </div>
    </Section>
  );
}
