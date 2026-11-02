const INK = 'var(--mascot-ink)';
const FUR = 'var(--mascot-fur)';

function CommitRing({ cx, cy }: { cx: number; cy: number }) {
  return (
    <>
      <circle cx={cx} cy={cy} r="8" fill={FUR} stroke={INK} strokeWidth="3" />
      <circle cx={cx} cy={cy} r="3" fill="var(--mascot-paper)" />
    </>
  );
}

export function MascotTail() {
  const main = 'M110 144 C134 146 142 136 142 118 V58';
  const branch = 'M142 116 C142 102 156 102 156 90 V86';

  return (
    <g data-part="tail" fill="none" strokeLinecap="round">
      <path d={main} stroke={INK} strokeWidth="13" />
      <path d={branch} stroke={INK} strokeWidth="13" />
      <path d={main} stroke={FUR} strokeWidth="8" />
      <path d={branch} stroke={FUR} strokeWidth="8" />
      <CommitRing cx={142} cy={50} />
      <CommitRing cx={156} cy={80} />
    </g>
  );
}

export function MascotBody() {
  return (
    <g data-part="body">
      <path
        d="M38 150 V110 C38 91 54 80 76 80 C98 80 114 91 114 110 V150 Z"
        fill={FUR}
        stroke={INK}
        strokeWidth="3"
      />
      <ellipse cx="76" cy="124" rx="20" ry="22" fill="var(--mascot-light)" />
    </g>
  );
}
