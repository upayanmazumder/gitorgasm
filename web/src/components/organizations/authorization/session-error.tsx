import { Button, StatusMessage } from '@/components/ui';

interface SessionErrorProps {
  unavailable: boolean;
  isRetrying: boolean;
  onRetry: () => void;
}

export function SessionError({ unavailable, isRetrying, onRetry }: SessionErrorProps) {
  return (
    <section className="space-y-4">
      <StatusMessage role="alert" tone="error">
        {unavailable
          ? 'Could not check your session. The service may be temporarily unavailable. Try again in a moment.'
          : 'Could not check your session. Try again.'}
      </StatusMessage>
      <Button variant="outline" onClick={onRetry} disabled={isRetrying}>
        Try again
      </Button>
    </section>
  );
}
