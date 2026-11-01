import { SpotlightCard } from '@/components/react-bits';

import { features } from './features';

export function HomeFeatures() {
  return (
    <section
      id="features"
      aria-labelledby="features-heading"
      className="scroll-mt-8 py-16 sm:py-20"
    >
      <h2 id="features-heading" className="text-3xl font-semibold tracking-tight sm:text-4xl">
        A clearer view of GitHub activity.
      </h2>
      <div className="mt-10 grid gap-6 md:grid-cols-3">
        {features.map(({ title, description }) => (
          <SpotlightCard key={title} className="p-6">
            <h3 className="text-lg font-semibold tracking-tight">{title}</h3>
            <p className="mt-3 text-sm leading-7 text-muted-foreground">{description}</p>
          </SpotlightCard>
        ))}
      </div>
    </section>
  );
}
