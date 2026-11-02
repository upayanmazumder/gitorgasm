import { BrandLink } from '@/components/brand';
import { LinkButton } from '@/components/ui';

import styles from './home.module.css';

export function HomeNavigation() {
  return (
    <header className={styles.header}>
      <nav aria-label="Main navigation" className={`${styles.container} ${styles.nav}`}>
        <BrandLink size={28} />
        <div className={styles.links}>
          <LinkButton href="#how-it-works" variant="link" className={styles.hideSmall}>
            How it works
          </LinkButton>
          <LinkButton href="/organizations" variant="outline">
            Open app
          </LinkButton>
        </div>
      </nav>
    </header>
  );
}
