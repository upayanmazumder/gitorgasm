import { cn } from '@/lib';

export type StatusTone = 'neutral' | 'success' | 'error';

interface Props {
  tone: StatusTone;
  className?: string;
}

function Shape({ tone }: Pick<Props, 'tone'>) {
  if (tone === 'success') {
    return (
      <>
        <circle cx="10" cy="10" r="8" />
        <path d="m6.5 10.2 2.4 2.4 4.6-5" />
      </>
    );
  }
  if (tone === 'error') {
    return (
      <>
        <circle cx="10" cy="10" r="8" />
        <path d="M10 6v4.5" />
        <path d="M10 13.6v.01" />
      </>
    );
  }
  return (
    <>
      <path d="M2 10h4.5" />
      <path d="M13.5 10H18" />
      <circle cx="10" cy="10" r="3.5" />
    </>
  );
}

export function StatusIcon({ tone, className }: Props) {
  return (
    <svg
      viewBox="0 0 20 20"
      width="20"
      height="20"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
      className={cn('shrink-0', className)}
    >
      <Shape tone={tone} />
    </svg>
  );
}
