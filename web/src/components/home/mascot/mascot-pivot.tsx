import type { ReactNode } from 'react';

interface Props {
  part: 'arm-wave' | 'head';
  x: number;
  y: number;
  children: ReactNode;
}

// The animated group sits between two translates, so its default origin is the pivot in any viewBox.
export function MascotPivot({ part, x, y, children }: Props) {
  return (
    <g transform={`translate(${x} ${y})`}>
      <g data-part={part}>
        <g transform={`translate(${-x} ${-y})`}>{children}</g>
      </g>
    </g>
  );
}
