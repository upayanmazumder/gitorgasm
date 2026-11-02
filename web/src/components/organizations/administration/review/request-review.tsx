import { Button, CheckboxField, StatusMessage } from '@/components/ui';

import {
  confirmLabel,
  describedFrozen,
  people,
  requiredAcknowledgment,
  type Frozen,
  type Phase,
} from '../draft';
import { RejectedAlert, UncertainAlert } from './enqueue-alert';
import { ReviewDetails } from './review-details';

type Register = (target: string) => (element: HTMLElement | null) => void;

function acknowledgmentLabel(request: Frozen, org: string) {
  const count = request.body.logins.length;
  return requiredAcknowledgment(request) === 'owner'
    ? `I understand that ${people(count)} will get complete administrative access to ${org}.`
    : `I understand that GitHub may invite ${people(request.unconfirmed.length)} to ${org}.`;
}

interface Props {
  org: string;
  phase: Extract<Phase, { request: Frozen }>;
  register: Register;
  onAcknowledge: (value: boolean) => void;
  onConfirm: () => void;
  onEdit: () => void;
  onResend: () => void;
  onCheckRecent: () => void;
}

export function RequestReview({
  org,
  phase,
  register,
  onAcknowledge,
  onConfirm,
  onEdit,
  onResend,
  onCheckRecent,
}: Props) {
  const { request } = phase;
  const submitting = phase.name === 'submitting';

  return (
    <div className="space-y-6">
      <ReviewDetails org={org} request={request} />
      {phase.name === 'reviewing' && requiredAcknowledgment(request) && (
        <div className="space-y-2">
          <CheckboxField
            label={acknowledgmentLabel(request, org)}
            checked={phase.acknowledged}
            ref={register('ack')}
            onChange={event => onAcknowledge(event.target.checked)}
          />
          {phase.ackError && (
            <StatusMessage role="alert" tone="error">
              Select the confirmation box to continue.
            </StatusMessage>
          )}
        </div>
      )}
      {phase.name === 'reviewing' && phase.rejection && (
        <RejectedAlert rejection={phase.rejection} org={org} register={register} />
      )}
      {phase.name === 'uncertain' && (
        <UncertainAlert
          rateLimited={phase.rateLimited}
          register={register}
          onResend={onResend}
          onCheckRecent={onCheckRecent}
          onDiscard={onEdit}
        />
      )}
      {submitting && <StatusMessage role="status">Sending the request to Gitorgasm…</StatusMessage>}
      {phase.name !== 'uncertain' && (
        <div className="flex flex-wrap gap-3">
          <Button
            className="h-auto min-h-11 max-w-full py-2 text-left wrap-anywhere whitespace-normal"
            onClick={onConfirm}
            disabled={submitting}
          >
            {confirmLabel(describedFrozen(request))}
          </Button>
          <Button variant="outline" onClick={onEdit} disabled={submitting}>
            Edit request
          </Button>
        </div>
      )}
    </div>
  );
}
