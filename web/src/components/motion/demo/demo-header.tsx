import { PageHeader } from '@/components/layout';
import { LinkButton } from '@/components/ui';

import { Fade } from '..';

export function DemoHeader() {
  return (
    <Fade>
      <PageHeader
        lead={
          <LinkButton href="/" variant="link">
            Back home
          </LinkButton>
        }
        title="Motion"
        description="Reusable Framer Motion primitives — LazyMotion-powered, tree-shakeable, and reduced-motion aware."
      />
    </Fade>
  );
}
