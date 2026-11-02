import type { ReactNode } from 'react';

import { panelVariants } from '@/components/ui';

interface Props {
  heading: string;
  body: string[];
  actions: ReactNode;
}

export function AccessNotice({ heading, body, actions }: Props) {
  return (
    <section
      aria-labelledby="people-access-heading"
      className={panelVariants({ className: 'space-y-4' })}
    >
      <h2 id="people-access-heading">{heading}</h2>
      {body.map(text => (
        <p key={text} className="max-w-[60ch]">
          {text}
        </p>
      ))}
      <div className="flex flex-wrap gap-3">{actions}</div>
    </section>
  );
}
