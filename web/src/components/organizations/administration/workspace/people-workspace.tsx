import { panelVariants } from '@/components/ui';

import type { Capability } from '../data';
import { RequestEditor } from '../editor';
import { RecentRequests } from '../history';
import { OperationProgress } from '../progress';
import { RequestReview } from '../review';
import { usePeopleWorkspace } from './use-people-workspace';

interface Props {
  login: string;
  capability: Capability;
}

export function PeopleWorkspace({ login, capability }: Props) {
  const workspace = usePeopleWorkspace(login, capability);
  const { state, receipt, operation, register, focus, dispatch } = workspace;
  const { phase, draft } = state;
  // Review again replaces the whole draft, so it needs an empty editor.
  const reviewAgainHelp =
    phase.name !== 'editing'
      ? 'Finish or edit the current request first.'
      : draft.people.length > 0
        ? 'Clear the selected people first.'
        : null;

  return (
    <>
      {receipt !== null && (
        <OperationProgress
          key={receipt}
          login={login}
          receipt={receipt}
          query={operation}
          reviewAgainHelp={reviewAgainHelp}
          onReviewAgain={workspace.reviewAgain}
          onClose={workspace.closeProgress}
          register={register}
        />
      )}
      <section
        aria-labelledby="people-request-heading"
        className={panelVariants({ className: 'space-y-6' })}
      >
        <h2 id="people-request-heading" tabIndex={-1} ref={register('request')}>
          {phase.name === 'editing' ? 'New request' : 'Review request'}
        </h2>
        {phase.name === 'editing' ? (
          <RequestEditor
            login={login}
            capability={capability}
            draft={draft}
            errors={phase.errors}
            dispatch={dispatch}
            onReview={workspace.review}
            focus={focus}
            register={register}
          />
        ) : (
          <RequestReview
            org={login}
            phase={phase}
            register={register}
            onAcknowledge={value => dispatch({ type: 'acknowledge', value })}
            onConfirm={workspace.confirm}
            onEdit={workspace.backToEditor}
            onResend={workspace.resend}
            onCheckRecent={workspace.checkRecent}
          />
        )}
      </section>
      <RecentRequests
        login={login}
        receipt={receipt}
        active={operation.data}
        onOpen={workspace.openReceipt}
        register={register}
      />
    </>
  );
}
