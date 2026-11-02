import { SunbeamField } from './effects';
import { HomeFeatures } from './features';
import { displayFont, textFont } from './fonts';
import { HomeFooter } from './footer';
import { HomeHero } from './hero';
import { HomeNavigation } from './home-navigation';
import styles from './home.module.css';
import { HomeScope } from './scope';

export function HomeView() {
  return (
    <div className={`home ${displayFont.variable} ${textFont.variable} ${styles.page}`}>
      <SunbeamField />
      <a href="#home-content" className={styles.skip}>
        Skip to content
      </a>
      <HomeNavigation />
      <main id="home-content" tabIndex={-1} className={styles.container}>
        <HomeHero />
        <HomeFeatures />
        <HomeScope />
      </main>
      <HomeFooter />
    </div>
  );
}
