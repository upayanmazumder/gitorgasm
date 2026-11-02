import type { ReactNode } from 'react';

import { Slide } from '..';

export function Section({
  title,
  hint,
  children,
}: {
  title: string;
  hint: string;
  children: ReactNode;
}) {
  return (
    <section className="space-y-6">
      <Slide inView className="space-y-1">
        <h2>{title}</h2>
        <p className="text-sm text-muted-foreground">{hint}</p>
      </Slide>
      {children}
    </section>
  );
}
