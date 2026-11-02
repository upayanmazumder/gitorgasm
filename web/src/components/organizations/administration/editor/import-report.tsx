import { StatusMessage } from '@/components/ui';

import { people as countPeople, type ImportError, type ImportIssue } from '../draft';

export type ImportSource = { kind: 'paste' } | { kind: 'file'; name: string };

export type ImportOutcome =
  | { error: ImportError; row?: number }
  | { invalid: ImportIssue[]; added: number; duplicates: number; total: number; overflow: boolean };

interface Props {
  source: ImportSource;
  outcome: ImportOutcome;
  max: number;
}

const SHOWN_ISSUES = 20;

function shorten(value: string) {
  return value.length > 40 ? `${value.slice(0, 40)}…` : value;
}

function errorText(error: ImportError, source: ImportSource, row?: number) {
  const name = source.kind === 'file' ? source.name : 'the pasted text';
  switch (error) {
    case 'too_large':
      return source.kind === 'file'
        ? `${name} is larger than 64 KiB. Split it into smaller files.`
        : 'Paste at most 64 KiB of text.';
    case 'wrong_type':
      return 'Choose a .csv or .json file.';
    case 'not_utf8':
      return `${name} is not UTF-8 text. Save it as UTF-8, then try again.`;
    case 'no_column':
      return `${name} has more than one column but no login, username, github_username, or github header.`;
    case 'unclosed_quote':
      return `${name} has a quote that does not close. It starts in row ${row}.`;
    case 'json_syntax':
      return `${name} is not valid JSON.`;
    case 'json_shape':
      return `${name} must contain one array of usernames.`;
    case 'empty':
      return `No usernames found in ${name}.`;
  }
}

export function ImportReport({ source, outcome, max }: Props) {
  const name = source.kind === 'file' ? source.name : 'the pasted text';
  if ('error' in outcome) {
    return (
      <StatusMessage role="alert" tone="error">
        {errorText(outcome.error, source, outcome.row)}
      </StatusMessage>
    );
  }
  const { invalid, added, duplicates, total, overflow } = outcome;
  return (
    <div className="space-y-2">
      {overflow && (
        <StatusMessage role="alert" tone="error">
          This would make {total} people. A request can include at most {max}. Remove people or
          split the list. Gitorgasm added no one from {name}.
        </StatusMessage>
      )}
      {!overflow && (added > 0 || duplicates > 0) && (
        <StatusMessage role="status" tone="success">
          Added {countPeople(added)} from {name}.
          {duplicates > 0 && ` ${duplicates} were already in the list.`}
        </StatusMessage>
      )}
      {invalid.length > 0 && (
        <>
          <StatusMessage role="alert" tone="error">
            {invalid.length} entries in {name} are not valid GitHub usernames. Gitorgasm did not add
            them.
          </StatusMessage>
          <ul className="list-disc pl-6 text-sm wrap-anywhere">
            {invalid.slice(0, SHOWN_ISSUES).map((issue, index) => (
              <li key={index}>
                {issue.position ? `${issue.position}: ` : ''}“{shorten(issue.value)}”
              </li>
            ))}
          </ul>
          {invalid.length > SHOWN_ISSUES && (
            <p className="text-sm">And {invalid.length - SHOWN_ISSUES} more.</p>
          )}
        </>
      )}
    </div>
  );
}
