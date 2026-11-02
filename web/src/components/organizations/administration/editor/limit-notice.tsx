import { StatusMessage } from '@/components/ui';

export function LimitNotice({ max }: { max: number }) {
  return (
    <StatusMessage role="status">
      You selected {max} people. A request can include at most {max}.
    </StatusMessage>
  );
}
