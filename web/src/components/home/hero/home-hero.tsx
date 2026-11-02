import { LinkButton } from '@/components/ui';

import { WavingMascot } from '../mascot';
import { ContributionScoreboard } from '../scoreboard';
import styles from './hero.module.css';

export function HomeHero() {
  return (
    <section aria-labelledby="home-heading" className={styles.hero}>
      <div className={styles.copy}>
        <h1 id="home-heading">Your GitHub organization’s activity, scored daily.</h1>
        <p className={styles.lead}>
          Connect an organization to see activity scores for its contributors and repositories.
          Choose which activity counts, pick a UTC date period, and save the views you check often.
        </p>
        <div className={styles.actions}>
          <LinkButton href="/organizations" size="lg">
            Connect an organization
          </LinkButton>
          <LinkButton href="#how-it-works" variant="outline" size="lg">
            How it works
          </LinkButton>
        </div>
        <p className={styles.note}>Sign in with GitHub to start.</p>
      </div>
      <div className={styles.product}>
        <div className={styles.mascot}>
          <WavingMascot />
        </div>
        <ContributionScoreboard />
      </div>
    </section>
  );
}
