import { ContributionDither } from './illustrations';

const features = [
  {
    title: 'Connect your organization',
    description:
      'Sign in with GitHub and install the GitHub App to bring accessible organization activity into one place.',
    variant: 'branches',
  },
  {
    title: 'Find your focus',
    description:
      'Explore contributor and repository activity scores. Narrow the view with date and time ranges, filters, and search.',
    variant: 'calendar',
  },
  {
    title: 'Return to what matters',
    description:
      'Save your query options as presets and load them again. Daily score refreshes keep your next review grounded in recent activity.',
    variant: 'history',
  },
] as const;

export function HomeFeatures() {
  return (
    <section
      id="features"
      aria-labelledby="features-heading"
      className="scroll-mt-8 border-t py-16 sm:py-20"
    >
      <p className="text-sm font-medium text-muted-foreground">Less digging. More context.</p>
      <h2 id="features-heading" className="mt-3 text-3xl font-semibold tracking-tight sm:text-4xl">
        A clearer view of GitHub activity.
      </h2>
      <div className="mt-10 grid gap-10 md:grid-cols-3 md:gap-8">
        {features.map(({ title, description, variant }) => (
          <article key={title}>
            <div className="mb-6 border-b pb-4">
              <ContributionDither variant={variant} className="h-32 w-full text-muted-foreground" />
            </div>
            <h3 className="text-lg font-semibold tracking-tight">{title}</h3>
            <p className="mt-3 text-sm leading-7 text-muted-foreground">{description}</p>
          </article>
        ))}
      </div>
    </section>
  );
}
