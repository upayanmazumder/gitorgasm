import styles from './scoreboard.module.css';

const contributors = [
  { name: 'ana-k', score: 92 },
  { name: 'jdoe', score: 71 },
  { name: 'mori', score: 58 },
  { name: 'kline', score: 44 },
];

export function ContributorRanking() {
  return (
    <ol
      className={styles.ranking}
      role="list"
      aria-label="Illustrative contributor activity scores"
    >
      {contributors.map(({ name, score }, index) => (
        <li key={name} className={styles.row}>
          <span className={styles.rank} aria-hidden="true">
            {index + 1}
          </span>
          <span className={styles.identity}>
            <span className={styles.name}>{name}</span>
            <span className={styles.track} aria-hidden="true">
              <span style={{ width: `${score}%` }} />
            </span>
          </span>
          <span className={styles.score}>
            {score}
            <span className="sr-only"> points</span>
          </span>
        </li>
      ))}
    </ol>
  );
}
