import { LinkButton } from '@/components/ui';

import { FilterPanel } from './filter-panel';
import styles from './scope.module.css';

export function HomeScope() {
  return (
    <section id="scope" aria-labelledby="scope-heading" className={styles.section}>
      <div className={styles.copy}>
        <h2 id="scope-heading">Decide what each score includes.</h2>
        <p>
          Filter by code, reviews, or issues. Set an exact UTC window, or use a preset period such
          as the last 7 or 30 days.
        </p>
        <p className={styles.boundary}>
          Scores summarize recorded GitHub activity. They do not measure productivity or the value
          of anyone’s work.
        </p>
        <LinkButton href="/organizations" size="lg">
          Connect an organization
        </LinkButton>
      </div>
      <FilterPanel />
    </section>
  );
}
