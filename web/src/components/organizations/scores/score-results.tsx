import type { ScoreResult, ScoreRow } from './types';

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
    <section className="space-y-2">
      <h2 className="text-xl font-semibold">{title}</h2>
      {rows.length === 0 ? (
        <p>No activity in this period.</p>
      ) : (
        <ol className="space-y-2">
          {rows.map(row => (
            <li key={label(row)} className="flex justify-between rounded border p-3">
              <span>
                {row.rank}. {label(row)}
              </span>
              <strong>{Number(row.score.toFixed(2))} points</strong>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

export function ScoreResults({ result }: Props) {
  return (
    <div className="space-y-6">
      <p role="status">
        {result.fresh
          ? 'Scores are current.'
          : result.state.ingestion_failed
            ? 'The last update failed. These scores may be old.'
            : 'Scores may be old. The latest update is not complete.'}
      </p>
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
