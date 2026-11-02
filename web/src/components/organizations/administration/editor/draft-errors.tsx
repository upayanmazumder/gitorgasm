import { Button, StatusMessage } from '@/components/ui';

import { people, type DraftError } from '../draft';

const MESSAGES: Record<Exclude<DraftError, 'blocked_people'>, string> = {
  no_people: 'Add at least one person.',
  no_team: 'Choose a team.',
  no_role: 'Choose a role.',
};

interface Props {
  errors: readonly DraftError[];
  blockedCount: number;
  onRemoveBlocked: () => void;
  register: (target: string) => (element: HTMLElement | null) => void;
}

export function DraftErrors({ errors, blockedCount, onRemoveBlocked, register }: Props) {
  if (errors.length === 0) return null;
  return (
    <div tabIndex={-1} ref={register('errors')} className="space-y-4">
      <StatusMessage role="alert" tone="error">
        Fix these problems, then review again.
      </StatusMessage>
      <ul className="list-disc space-y-2 pl-6">
        {errors.map(error =>
          error === 'blocked_people' ? (
            <li key={error}>
              Remove {people(blockedCount)} who cannot be part of this request.
              <div className="pt-2">
                <Button variant="outline" onClick={onRemoveBlocked}>
                  Remove {people(blockedCount)}
                </Button>
              </div>
            </li>
          ) : (
            <li key={error}>{MESSAGES[error]}</li>
          )
        )}
      </ul>
    </div>
  );
}
