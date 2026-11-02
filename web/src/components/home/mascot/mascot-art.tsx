import { MascotBody, MascotTail } from './mascot-body';
import { MascotHead } from './mascot-head';
import { MascotLimb } from './mascot-limb';
import { MascotPivot } from './mascot-pivot';
import styles from './mascot.module.css';

interface Props {
  playing?: boolean;
}

// Drawn bottom to top; the waving arm comes last so it passes in front of the face.
export function MascotArt({ playing = false }: Props) {
  return (
    <svg
      className={styles.art}
      viewBox="-16 0 192 160"
      width="192"
      height="160"
      aria-hidden="true"
      focusable="false"
      data-playing={playing}
    >
      <MascotTail />
      <MascotBody />
      <g data-part="leg-static">
        <MascotLimb line="M100 100 L92 142" pawX={92} />
      </g>
      <MascotPivot part="head" x={76} y={96}>
        <MascotHead />
      </MascotPivot>
      <MascotPivot part="arm-wave" x={52} y={100}>
        <MascotLimb line="M52 100 L60 142" pawX={60} />
      </MascotPivot>
    </svg>
  );
}
