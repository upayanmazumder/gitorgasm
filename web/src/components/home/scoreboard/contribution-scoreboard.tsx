import { SpotlightPanel } from '../effects';
import { ContributorRanking } from './contributor-ranking';
import { RepositorySignal } from './repository-signal';
import styles from './scoreboard.module.css';

export function ContributionScoreboard() {
  return (
    <figure className={styles.figure}>
      <SpotlightPanel>
        <div className={styles.toolbar}>
          <span className={styles.org}>example-org</span>
          <span className={styles.pill}>Illustrative data</span>
        </div>
        <div className={styles.heading}>
          <h2>Contributor scores</h2>
          <p>Last 30 days, UTC</p>
        </div>
        <ContributorRanking />
        <RepositorySignal />
        <p className={styles.refresh}>Scores refresh daily.</p>
      </SpotlightPanel>
      <figcaption className={styles.caption}>
        Sample figures for illustration. Not live data.
      </figcaption>
    </figure>
  );
}
