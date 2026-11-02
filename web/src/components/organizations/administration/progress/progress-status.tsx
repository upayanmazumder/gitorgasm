import { Button, LinkButton, StatusMessage } from '@/components/ui';

import type { Operation } from '../data';
import { people } from '../draft';
import { countDone, countStatus, hasProblems, summarize } from './result-copy';

interface Line {
  role: 'status' | 'alert';
  tone: 'neutral' | 'success' | 'error';
  text: string;
}

function lineOf(operation: Operation): Line {
  const summary = summarize(operation);
  switch (operation.status) {
    case 'queued':
      return { role: 'status', tone: 'neutral', text: 'Waiting to start.' };
    case 'running':
      return {
        role: 'status',
        tone: 'neutral',
        text: 'In progress. Gitorgasm makes one change at a time.',
      };
    case 'interrupted':
      return { role: 'alert', tone: 'error', text: `Stopped before it finished. ${summary}.` };
    case 'completed':
      return hasProblems(operation)
        ? { role: 'alert', tone: 'error', text: `Finished with problems. ${summary}.` }
        : { role: 'status', tone: 'success', text: `Finished. ${summary}.` };
  }
}

// One element stays mounted and only its text changes, so each state change is announced once.
export function StatusLine({ operation }: { operation: Operation }) {
  const { role, tone, text } = lineOf(operation);
  return (
    <div className="space-y-2">
      <StatusMessage role={role} tone={tone}>
        {text}
      </StatusMessage>
      {operation.status === 'running' && (
        <p className="tabular-nums">
          {countDone(operation)} of {operation.results.length} done
        </p>
      )}
    </div>
  );
}

interface NotesProps {
  operation: Operation;
  org: string;
  reviewAgainHelp: string | null;
  onReviewAgain: () => void;
}

export function ResultNotes({ operation, org, reviewAgainHelp, onReviewAgain }: NotesProps) {
  const unknown = countStatus(operation, 'unknown');
  const notAttempted = countStatus(operation, 'not_attempted');
  const invited =
    countStatus(operation, 'invited') > 0 ||
    (operation.kind === 'invite' && countStatus(operation, 'succeeded') > 0);
  return (
    <div className="max-w-[60ch] space-y-4">
      {unknown > 0 && (
        <div className="space-y-2">
          <p>
            GitHub may have applied {unknown} {unknown === 1 ? 'change' : 'changes'}, but Gitorgasm
            could not confirm {unknown === 1 ? 'it' : 'them'}. Check these people on GitHub before
            you try again.
          </p>
          <LinkButton
            href={`https://github.com/orgs/${encodeURIComponent(org)}/people`}
            native
            variant="link"
            target="_blank"
            rel="noopener noreferrer"
            className="h-auto min-h-11 max-w-full py-2 text-left wrap-anywhere whitespace-normal"
          >
            Open {org} people on GitHub
          </LinkButton>
        </div>
      )}
      {notAttempted > 0 && (
        <div className="space-y-2">
          <p>Gitorgasm did not try {people(notAttempted)} because the request stopped.</p>
          <Button
            variant="outline"
            onClick={onReviewAgain}
            disabled={reviewAgainHelp !== null}
            aria-describedby={reviewAgainHelp === null ? undefined : 'people-review-again-help'}
          >
            Review again for {people(notAttempted)}
          </Button>
          {reviewAgainHelp !== null && (
            <p id="people-review-again-help" className="text-sm text-muted-foreground">
              {reviewAgainHelp}
            </p>
          )}
        </div>
      )}
      {invited && <p>Invited people join {org} after they accept the invitation on GitHub.</p>}
    </div>
  );
}
