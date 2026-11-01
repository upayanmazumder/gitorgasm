import { Button } from '@/components/ui';

interface SessionErrorProps {
  onRetry: () => void;
}

export function SessionError({ onRetry }: SessionErrorProps) {
  return (
    <section className="space-y-4">
      <p role="alert">Could not check your session. Check your connection and try again.</p>
      <Button variant="outline" onClick={onRetry}>
        Try again
      </Button>
    </section>
  );
}
