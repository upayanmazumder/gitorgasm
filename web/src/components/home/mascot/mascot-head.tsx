const INK = 'var(--mascot-ink)';
const FUR = 'var(--mascot-fur)';
const SHADE = 'var(--mascot-shade)';

const EARS = [
  'M36 52 L42 16 Q43.5 10 48.5 13.5 L74 38 Z',
  // The notch on the viewer's right ear tells this cat apart at a glance.
  'M116 52 L113.3 35.8 L109 32.5 L112.4 30.4 L110 16 Q108.5 10 103.5 13.5 L78 38 Z',
];

export function MascotHead() {
  return (
    <g strokeLinecap="round" strokeLinejoin="round">
      {EARS.map(d => (
        <path key={d} d={d} fill={FUR} stroke={INK} strokeWidth="3" />
      ))}
      <path d="M44 42 L47 22 L64 38 Z" fill={SHADE} />
      <path d="M108 42 L105 22 L88 38 Z" fill={SHADE} />
      <rect x="30" y="34" width="92" height="62" rx="28" fill={FUR} stroke={INK} strokeWidth="3" />
      <ellipse cx="76" cy="80" rx="19" ry="12" fill="var(--mascot-light)" />
      <g data-part="eyes-open" fill={INK}>
        <rect x="56" y="54" width="9" height="14" rx="4.5" />
        <rect x="87" y="54" width="9" height="14" rx="4.5" />
      </g>
      <path
        data-part="eyes-happy"
        d="M54 63 Q60.5 55 67 63 M85 63 Q91.5 55 98 63"
        fill="none"
        stroke={INK}
        strokeWidth="4.5"
        opacity="0"
      />
      <path
        d="M71.5 73 h9 a1.5 1.5 0 0 1 1.2 2.4 l-4.5 4.6 a1.6 1.6 0 0 1 -2.4 0 l-4.5 -4.6 a1.5 1.5 0 0 1 1.2 -2.4 Z"
        fill={INK}
      />
      <path d="M68 82 Q72 87 76 82 Q80 87 84 82" fill="none" stroke={INK} strokeWidth="3" />
      <path
        d="M50 80 L32 77 M50 86 L33 89 M102 80 L120 77 M102 86 L119 89"
        fill="none"
        stroke={INK}
        strokeWidth="2.5"
      />
    </g>
  );
}
