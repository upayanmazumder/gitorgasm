import { Dither } from '@/components/react-bits';

// Rendered dark, then inverted in light mode so one palette serves both themes.
export function HeroDither() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none absolute inset-0 -z-10 [mask-image:radial-gradient(ellipse_at_75%_25%,black,transparent_70%)] opacity-30 invert dark:invert-0"
    >
      <Dither
        ditherMode="bayer"
        waveColor={[0.6, 0.6, 0.6]}
        backgroundColor={[0.02, 0.02, 0.02]}
        colorNum={2}
        pixelSize={4}
        waveAmplitude={0.4}
        waveSpeed={0.03}
        enableMouseInteraction={false}
      />
    </div>
  );
}
