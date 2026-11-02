import { ButtonLink } from './button-link';
import { HomeBrand } from './home-brand';
import styles from './home.module.css';

export function HomeNavigation() {
  return (
    <header className={styles.header}>
      <nav aria-label="Main navigation" className={`${styles.container} ${styles.nav}`}>
        <HomeBrand size={28} />
        <div className={styles.links}>
          <ButtonLink href="#how-it-works" variant="text" className={styles.hideSmall}>
            How it works
          </ButtonLink>
          <ButtonLink href="/organizations" variant="secondary" compact>
            Open app
          </ButtonLink>
        </div>
      </nav>
    </header>
  );
}
