const INK = 'var(--mascot-ink)';
const FUR = 'var(--mascot-fur)';

interface Props {
  line: string;
  pawX: number;
}

export function MascotLimb({ line, pawX }: Props) {
  return (
    <g fill="none" strokeLinecap="round">
      <path d={line} stroke={INK} strokeWidth="22" />
      <path d={line} stroke={FUR} strokeWidth="16" />
      <ellipse cx={pawX} cy="146" rx="11" ry="7" fill={FUR} stroke={INK} strokeWidth="3" />
      <path d={`M${pawX - 4} 143 v4 M${pawX + 4} 143 v4`} stroke={INK} strokeWidth="2" />
    </g>
  );
}
