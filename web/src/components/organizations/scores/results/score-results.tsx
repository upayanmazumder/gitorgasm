import { panelVariants, StatusMessage } from '@/components/ui';

import type { ScoreResult, ScoreRow } from '../data';

interface Props {
  result: ScoreResult;
}

function Board({
  title,
  rows,
  label,
}: {
  title: string;
  rows: ScoreRow[];
  label: (row: ScoreRow) => string;
}) {
  return (
    <section className={panelVariants({ className: 'space-y-4' })}>
      <h2>{title}</h2>
      {rows.length === 0 ? (
        <p className="text-muted-foreground">No activity in this period.</p>
      ) : (
        <ol>
          {rows.map(row => (
            <li
              key={label(row)}
              className="flex items-baseline justify-between gap-4 border-b border-border py-3 last:border-b-0"
            >
              <span>
                {row.rank}. {label(row)}
              </span>
              <strong className="font-semibold tabular-nums">
                {Number(row.score.toFixed(2))} points
              </strong>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

export function ScoreResults({ result }: Props) {
  const tone = result.fresh ? 'success' : result.state.ingestion_failed ? 'error' : 'neutral';

  return (
    <div className="space-y-6">
      <StatusMessage role="status" tone={tone}>
        {result.fresh
          ? 'Scores are current.'
          : result.state.ingestion_failed
            ? 'The last update failed. These scores may be old.'
            : 'Scores may be old. The latest update is not complete.'}
      </StatusMessage>
      <Board
        title="Contributors"
        rows={result.data.contributors}
        label={row => row.userLogin ?? String(row.userId)}
      />
      <Board
        title="Repositories"
        rows={result.data.repositories}
        label={row => row.repoName ?? ''}
      />
    </div>
  );
}
