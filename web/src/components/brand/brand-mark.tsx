import {
  MARK_BODY,
  MARK_EARS,
  MARK_GLYPH_TRANSFORM,
  MARK_HEAD,
  MARK_RING,
  MARK_TAIL,
  MARK_VIEWBOX,
} from './mark-paths';

interface Props {
  size?: number;
  className?: string;
  title?: string;
}

export function BrandMark({ size = 28, className, title }: Props) {
  return (
    <svg
      viewBox={MARK_VIEWBOX}
      width={size}
      height={size}
      className={className}
      focusable="false"
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      <rect width="32" height="32" rx="7.5" fill="var(--mascot-fur)" />
      <g fill="var(--mascot-ink)" transform={MARK_GLYPH_TRANSFORM}>
        {MARK_EARS.map(d => (
          <path key={d} d={d} />
        ))}
        <path d={MARK_HEAD} fillRule="evenodd" />
        <path d={MARK_BODY} />
      </g>
      <g
        fill="none"
        stroke="var(--mascot-ink)"
        strokeLinecap="round"
        transform={MARK_GLYPH_TRANSFORM}
      >
        <path d={MARK_TAIL} strokeWidth="2.4" />
        <circle {...MARK_RING} strokeWidth="2.2" />
      </g>
    </svg>
  );
}
