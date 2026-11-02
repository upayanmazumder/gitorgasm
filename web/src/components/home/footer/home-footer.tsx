import { BrandLink } from '@/components/brand';
import { LinkButton } from '@/components/ui';

import homeStyles from '../home.module.css';
import styles from './footer.module.css';

export function HomeFooter() {
  return (
    <footer className={styles.footer}>
      <div className={`${homeStyles.container} ${styles.inner}`}>
        <div className={styles.about}>
          <BrandLink size={24} />
          <p>Activity scores for GitHub organizations.</p>
        </div>
        <nav aria-label="Footer navigation" className={styles.links}>
          <LinkButton href="#how-it-works" variant="link">
            How it works
          </LinkButton>
          <LinkButton href="/organizations" variant="link">
            Organizations
          </LinkButton>
          <LinkButton href="https://github.com/upayanmazumder/gitorgasm" variant="link">
            GitHub
          </LinkButton>
        </nav>
        <div className={styles.legal}>
          <p>Gitorgasm is not affiliated with GitHub.</p>
          <p>© 2026 Gitorgasm</p>
        </div>
      </div>
    </footer>
  );
}
