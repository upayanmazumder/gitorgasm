import styles from './scoreboard.module.css';

export function RepositorySignal() {
  return (
    <div className={styles.repository}>
      <div className={styles.repositoryText}>
        <p className={styles.repositoryLabel}>Repository activity</p>
        <p className={styles.repositoryName}>example-org/core</p>
      </div>
      <svg viewBox="0 0 180 44" className={styles.sparkline} aria-hidden="true" fill="none">
        <path
          d="m2 35 12-4 12 3 12-15 12 6 12-13 12 10 12-4 12 8 12-15 12 3 12-8 12 5 12-7 10 2"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinejoin="round"
          strokeLinecap="round"
        />
      </svg>
      <span className="sr-only">Illustrative repository activity trend.</span>
    </div>
  );
}
