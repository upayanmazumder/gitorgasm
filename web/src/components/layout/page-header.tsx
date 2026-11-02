import type { ReactNode } from 'react';

interface Props {
  lead: ReactNode;
  title: string;
  description?: string;
}

export function PageHeader({ lead, title, description }: Props) {
  return (
    <header className="space-y-4">
      {lead}
      <h1>{title}</h1>
      {description ? <p className="max-w-[60ch] text-muted-foreground">{description}</p> : null}
    </header>
  );
}
