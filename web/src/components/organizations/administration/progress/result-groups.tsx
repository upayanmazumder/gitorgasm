import { LinkButton } from '@/components/ui';

import type { Operation } from '../data';
import { groupResults } from './result-copy';

export function ResultGroups({ operation }: { operation: Operation }) {
  const groups = groupResults(operation);
  return (
    <>
      <nav aria-label="Result groups" className="flex flex-wrap gap-x-6 gap-y-2">
        {groups.map(group => (
          <LinkButton
            key={group.status}
            href={`#people-result-${group.status}`}
            native
            variant="link"
            className="tabular-nums"
          >
            {group.results.length} {group.label}
          </LinkButton>
        ))}
      </nav>
      {groups.map(group => (
        <div key={group.status} className="space-y-2 border-t border-border pt-6">
          <h3 id={`people-result-${group.status}`} tabIndex={-1}>
            {group.label} ({group.results.length})
          </h3>
          <ul aria-labelledby={`people-result-${group.status}`}>
            {group.results.map(result => (
              <li key={result.login} className="py-2">
                <span className="font-semibold wrap-anywhere">{result.login}</span>
                {result.message && (
                  <span className="block text-sm text-muted-foreground">{result.message}</span>
                )}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </>
  );
}
