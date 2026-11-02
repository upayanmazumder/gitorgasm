import styles from './features.module.css';

const steps = [
  {
    title: 'Connect an organization',
    description:
      'Sign in with GitHub and install the GitHub App on your organization. Gitorgasm shows the contributors and repositories that the app can access.',
  },
  {
    title: 'Choose what counts',
    description:
      'Pick an activity type and a UTC date period. Hide bots if you want, then save the combinations you check often as presets.',
  },
  {
    title: 'Come back to fresh scores',
    description:
      'Scores update daily, so each visit reflects recent activity in your organization.',
  },
];

export function HomeFeatures() {
  return (
    <section id="how-it-works" aria-labelledby="workflow-heading" className={styles.section}>
      <h2 id="workflow-heading">How it works</h2>
      <p className={styles.intro}>
        Three steps from a GitHub organization to scores you can return to.
      </p>
      <ol className={styles.steps} role="list">
        {steps.map(({ title, description }, index) => (
          <li key={title} className={styles.step}>
            <span className={styles.node} aria-hidden="true">
              {index + 1}
            </span>
            <h3>{title}</h3>
            <p>{description}</p>
          </li>
        ))}
      </ol>
    </section>
  );
}
