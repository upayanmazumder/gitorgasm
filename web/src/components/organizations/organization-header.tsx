import { BrandLink } from '@/components/brand';
import { PageHeader } from '@/components/layout';

export function OrganizationHeader() {
  return (
    <PageHeader
      lead={<BrandLink size={28} />}
      title="Your organizations"
      description="Connect GitHub to explore activity scores for the organizations you can access."
    />
  );
}
