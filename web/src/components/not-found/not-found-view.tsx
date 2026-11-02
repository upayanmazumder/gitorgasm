import { BrandLink } from '@/components/brand';
import { PageHeader, PageShell } from '@/components/layout';
import { LinkButton } from '@/components/ui';

export function NotFoundView() {
  return (
    <PageShell>
      <PageHeader
        lead={<BrandLink size={28} />}
        title="Page not found"
        description="No page exists at this address. Check the link, or go to your organizations."
      />
      <div>
        <LinkButton href="/organizations">Go to your organizations</LinkButton>
      </div>
    </PageShell>
  );
}
