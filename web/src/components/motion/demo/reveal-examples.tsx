import type { SlideDirection } from '@/lib';

import { Fade, Scale, Slide, Stagger, StaggerItem } from '..';
import { Card } from './card';
import { Section } from './section';

const DIRECTIONS: SlideDirection[] = ['up', 'down', 'left', 'right'];
const CARDS = ['01', '02', '03', '04', '05', '06'];

export function RevealExamples() {
  return (
    <>
      <Section title="Fade" hint="Opacity-only reveal on scroll.">
        <Fade inView>
          <Card>A block that fades in when it enters the viewport.</Card>
        </Fade>
      </Section>

      <Section
        title="Slide"
        hint="Directional slide + fade. Movement is dropped under reduced-motion."
      >
        <div className="grid grid-cols-2 gap-4">
          {DIRECTIONS.map((direction, i) => (
            <Slide key={direction} inView direction={direction} delay={i * 0.05}>
              <Card className="text-center capitalize">{direction}</Card>
            </Slide>
          ))}
        </div>
      </Section>

      <Section title="Scale" hint="Scale + fade reveal.">
        <Scale inView>
          <Card>A block that scales up as it fades in.</Card>
        </Scale>
      </Section>

      <Section title="Staggered cards" hint="Children animate in sequence from one parent.">
        <Stagger inView className="grid grid-cols-2 gap-4 md:grid-cols-3">
          {CARDS.map(label => (
            <StaggerItem key={label}>
              <Card className="text-center text-sm text-muted-foreground tabular-nums">
                {label}
              </Card>
            </StaggerItem>
          ))}
        </Stagger>
      </Section>
    </>
  );
}
