import { Button, StatusMessage } from '@/components/ui';

interface SessionErrorProps {
  onRetry: () => void;
}

export function SessionError({ onRetry }: SessionErrorProps) {
  return (
    <section className="space-y-4">
      <StatusMessage role="alert" tone="error">
        Could not check your session. Check your connection and try again.
      </StatusMessage>
      <Button variant="outline" onClick={onRetry}>
        Try again
      </Button>
    </section>
  );
}
