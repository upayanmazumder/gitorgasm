import styles from './scope.module.css';

const segments = ['All', 'Code', 'Reviews', 'Issues'];

export function FilterPanel() {
  return (
    <figure className={styles.figure}>
      <div aria-hidden="true" className={styles.panel}>
        <div className={styles.panelHeading}>
          <p className={styles.panelTitle}>Activity filters</p>
          <span className={styles.pill}>Illustrative view</span>
        </div>
        <div className={styles.field}>
          <p className={styles.label}>Activity</p>
          <div className={styles.segments}>
            {segments.map(segment => (
              <span key={segment} data-selected={segment === 'Reviews'}>
                {segment}
              </span>
            ))}
          </div>
        </div>
        <div className={styles.field}>
          <p className={styles.label}>Time period (UTC)</p>
          <div className={styles.select}>
            Last 30 days
            <svg viewBox="0 0 16 16" fill="none" width="16" height="16">
              <path
                d="m4 6 4 4 4-4"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>
        </div>
        <div className={styles.option}>
          <svg viewBox="0 0 20 20" fill="none" width="20" height="20">
            <rect x="1" y="1" width="18" height="18" rx="5" className={styles.checkBox} />
            <path
              d="m6 10.2 2.8 2.8L14 7.4"
              stroke="var(--card)"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          Hide bots
        </div>
        <div className={styles.option}>
          <svg viewBox="0 0 16 16" fill="none" width="20" height="20">
            <path
              d="M4 2.5h8v11l-4-3-4 3v-11Z"
              stroke="currentColor"
              strokeWidth="1.3"
              strokeLinejoin="round"
            />
          </svg>
          <span className={styles.label}>Saved preset</span>
          <strong>Monthly review</strong>
        </div>
      </div>
      <figcaption className={styles.caption}>
        Example filter settings, not a working form.
      </figcaption>
    </figure>
  );
}
