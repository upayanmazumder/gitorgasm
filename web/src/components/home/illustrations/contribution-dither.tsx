interface Props {
  variant?: 'calendar' | 'branches' | 'history';
  className?: string;
}

const bayer = [0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5];
const dotPaths = [3, 7, 11, 16].map(threshold =>
  bayer
    .flatMap((value, index) =>
      value < threshold ? [`M${(index % 4) * 4} ${Math.floor(index / 4) * 4}h2v2h-2z`] : []
    )
    .join(' ')
);
const cells = Array.from({ length: 84 }, (_, index) => ({
  x: 20 + Math.floor(index / 7) * 22,
  y: 20 + (index % 7) * 22,
  density: (Math.floor(index / 7) * 3 + (index % 7) * 5) % 4,
}));

export function ContributionDither({ variant = 'calendar', className }: Props) {
  return (
    <svg
      aria-hidden="true"
      focusable="false"
      viewBox="0 0 300 190"
      width="300"
      height="190"
      className={className}
      fill="currentColor"
    >
      {variant === 'calendar' ? (
        cells.map(({ x, y, density }) => (
          <path key={`${x}-${y}`} d={dotPaths[density]} transform={`translate(${x} ${y})`} />
        ))
      ) : (
        <>
          <g fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M42 36v118M42 96c0-42 108 10 108-38V36M42 130c0-42 216 12 216-38V70" />
            {[36, 96, 154].map(y => (
              <circle key={y} cx="42" cy={y} r="7" className="fill-background" />
            ))}
            <circle cx="150" cy="36" r="7" className="fill-background" />
            <circle cx="258" cy="70" r="7" className="fill-background" />
          </g>
          {[0, 1, 2, 3, 4, 5].map(index => (
            <path
              key={index}
              d={dotPaths[(index + (variant === 'history' ? 2 : 0)) % 4]}
              transform={`translate(${72 + index * 30} 150)`}
            />
          ))}
        </>
      )}
    </svg>
  );
}
