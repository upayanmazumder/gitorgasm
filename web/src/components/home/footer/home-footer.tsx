import { ButtonLink } from '../button-link';
import { HomeBrand } from '../home-brand';
import homeStyles from '../home.module.css';
import styles from './footer.module.css';

export function HomeFooter() {
  return (
    <footer className={styles.footer}>
      <div className={`${homeStyles.container} ${styles.inner}`}>
        <div className={styles.about}>
          <HomeBrand size={24} />
          <p>Activity scores for GitHub organizations.</p>
        </div>
        <nav aria-label="Footer navigation" className={styles.links}>
          <ButtonLink href="#how-it-works" variant="text">
            How it works
          </ButtonLink>
          <ButtonLink href="/organizations" variant="text">
            Organizations
          </ButtonLink>
          <ButtonLink href="https://github.com/upayanmazumder/gitorgasm" variant="text">
            GitHub
          </ButtonLink>
        </nav>
        <div className={styles.legal}>
          <p>Gitorgasm is not affiliated with GitHub.</p>
          <p>© 2026 Gitorgasm</p>
        </div>
      </div>
    </footer>
  );
}
