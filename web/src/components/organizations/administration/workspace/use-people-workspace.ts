import { useReducer } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { parseAsString, useQueryState } from 'nuqs';

import {
  accessKeys,
  classifyEnqueueError,
  isActive,
  operationDetailKey,
  operationKeys,
  useEnqueueOperation,
  useOperation,
  type Capability,
  type Operation,
} from '../data';
import {
  buildRequest,
  initialWorkspace,
  requiredAcknowledgment,
  retryDraft,
  validateDraft,
  workspaceReducer,
  type Frozen,
} from '../draft';
import { usePendingFocus } from './use-pending-focus';

const receiptParser = parseAsString.withOptions({ history: 'replace' });

export function usePeopleWorkspace(login: string, capability: Capability) {
  const client = useQueryClient();
  const [state, dispatch] = useReducer(workspaceReducer, initialWorkspace);
  const [receipt, setReceipt] = useQueryState('receipt', receiptParser);
  const operation = useOperation(login, receipt);
  const enqueue = useEnqueueOperation(login);
  const { focus, register } = usePendingFocus();

  function send(request: Frozen, idempotencyKey: string) {
    enqueue.mutate(
      { idempotencyKey, body: request.body },
      {
        onSuccess: created => {
          client.setQueryData(operationDetailKey(login, created.id), created);
          void setReceipt(created.id);
          dispatch({ type: 'enqueued' });
          void client.resetQueries({ queryKey: operationKeys.list({ login }) });
          focus('progress');
        },
        onError: error => {
          const failure = classifyEnqueueError(error);
          if (failure.kind === 'uncertain') {
            dispatch({ type: 'uncertain', rateLimited: failure.rateLimited });
          } else {
            dispatch({ type: 'rejected', rejection: failure.rejection });
            if (failure.rejection === 'access_changed') {
              void client.invalidateQueries({ queryKey: accessKeys.detail(login) });
            }
          }
          focus('enqueue-alert');
        },
      }
    );
  }

  function review() {
    const errors = validateDraft(state.draft, capability.actor);
    const frozen = errors.length === 0 ? buildRequest(state.draft) : null;
    dispatch({ type: 'review', errors, frozen });
    focus(frozen ? 'request' : 'errors');
  }

  function confirm() {
    const { phase } = state;
    if (phase.name !== 'reviewing') return;
    if (requiredAcknowledgment(phase.request) && !phase.acknowledged) {
      dispatch({ type: 'ackMissing' });
      focus('ack');
      return;
    }
    // The key is created here, once, so a re-render never produces a second key for the same request.
    const idempotencyKey = crypto.randomUUID();
    dispatch({ type: 'submit', idempotencyKey });
    send(phase.request, idempotencyKey);
  }

  function resend() {
    const { phase } = state;
    if (phase.name !== 'uncertain') return;
    dispatch({ type: 'resend' });
    send(phase.request, phase.idempotencyKey);
  }

  function backToEditor() {
    dispatch({ type: state.phase.name === 'uncertain' ? 'discard' : 'edit' });
    focus('request');
  }

  function checkRecent() {
    void client.invalidateQueries({ queryKey: operationKeys.list({ login }) });
    focus('history');
  }

  function openReceipt(id: string) {
    void setReceipt(id);
    focus('progress');
  }

  function closeProgress() {
    // A running receipt stops being observed here, so its history row would keep the old status.
    if (operation.data && isActive(operation.data)) {
      void client.invalidateQueries({ queryKey: operationKeys.list({ login }) });
    }
    void setReceipt(null);
    focus('request');
  }

  function reviewAgain(source: Operation) {
    dispatch({ type: 'loadRetry', draft: retryDraft(source) });
    focus('request');
  }

  return {
    state,
    dispatch,
    receipt,
    operation,
    register,
    focus,
    review,
    confirm,
    resend,
    backToEditor,
    checkRecent,
    openReceipt,
    closeProgress,
    reviewAgain,
  };
}
