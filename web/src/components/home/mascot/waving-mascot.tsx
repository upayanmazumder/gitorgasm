'use client';

import { useEffect, useRef, useState } from 'react';

import { MascotArt } from './mascot-art';
import styles from './mascot.module.css';
import { useReducedMotionPreference } from './use-reduced-motion-preference';

const VISIBLE_SHARE = 0.6;

export function WavingMascot() {
  const reduced = useReducedMotionPreference();
  const rootRef = useRef<HTMLDivElement>(null);
  const [playing, setPlaying] = useState(false);
  const [run, setRun] = useState(0);

  useEffect(() => {
    const root = rootRef.current;
    if (!root || reduced || playing) return;
    // Mobile can start with the cat below the fold, so the first wave waits until it shows.
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting && entry.intersectionRatio >= VISIBLE_SHARE) setPlaying(true);
      },
      { threshold: VISIBLE_SHARE }
    );
    observer.observe(root);
    return () => observer.disconnect();
  }, [reduced, playing]);

  if (reduced) {
    return (
      <div role="img" aria-label="The Gitorgasm cat waving hello" className={styles.root}>
        <MascotArt />
      </div>
    );
  }

  return (
    <div ref={rootRef} className={styles.root}>
      <button
        type="button"
        aria-label="Make the Gitorgasm cat wave"
        className={styles.button}
        onClick={() => {
          setPlaying(true);
          // A new key remounts the SVG, which restarts the CSS animation from 0%.
          setRun(count => count + 1);
        }}
      >
        <MascotArt key={run} playing={playing} />
      </button>
    </div>
  );
}
