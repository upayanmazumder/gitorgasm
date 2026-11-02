import { LinkButton } from '@/components/ui';

interface Props {
  login: string;
  current: 'scores' | 'people';
}

export function OrganizationNav({ login, current }: Props) {
  const base = `/organizations/${encodeURIComponent(login)}`;
  const sections = [
    { id: 'scores', label: 'Scores', href: base },
    { id: 'people', label: 'People', href: `${base}/people` },
  ] as const;

  return (
    <nav aria-label="Organization sections">
      <ul className="flex flex-wrap gap-3">
        {sections.map(section => (
          <li key={section.id}>
            <LinkButton
              href={section.href}
              variant={current === section.id ? 'outline' : 'link'}
              aria-current={current === section.id ? 'page' : undefined}
            >
              {section.label}
            </LinkButton>
          </li>
        ))}
      </ul>
    </nav>
  );
}
