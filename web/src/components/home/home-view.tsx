import { HomeFeatures } from './home-features';
import { HomeHero } from './home-hero';
import { HomeNavigation } from './home-navigation';

export function HomeView() {
  return (
    <div className="flex flex-1 flex-col bg-background text-foreground">
      <a
        href="#home-content"
        className="sr-only rounded-md bg-foreground p-3 text-background focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50"
      >
        Skip to content
      </a>
      <HomeNavigation />
      <main id="home-content" tabIndex={-1} className="mx-auto w-full max-w-6xl px-6 sm:px-10">
        <HomeHero />
        <HomeFeatures />
      </main>
      <footer className="mt-auto border-t">
        <div className="mx-auto flex max-w-6xl flex-col gap-2 px-6 py-6 text-xs text-muted-foreground sm:flex-row sm:justify-between sm:px-10">
          <p>Gitorgasm</p>
          <p>Organization activity. A little easier to read.</p>
        </div>
      </footer>
    </div>
  );
}
