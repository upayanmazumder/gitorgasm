export interface ScoreRow {
  userId?: number;
  userLogin?: string;
  repoName?: string;
  score: number;
  rank: number;
}

export interface ScoreResult {
  data: { contributors: ScoreRow[]; repositories: ScoreRow[]; teams: ScoreRow[] };
  state: { generation: number; last_ingested_at: string | null; ingestion_failed: boolean };
  fresh: boolean;
}

export interface ScoreQuery {
  period?: { kind: string; preset?: string; start?: string; end?: string };
  filter?: {
    preset?: string;
    search?: string | null;
    users?: number[];
    repositories?: string[];
    kinds?: string[];
    excludeBots?: boolean;
  };
  rules?: object;
}

export interface Preset {
  name: string;
  query: ScoreQuery;
}

export interface ScoreForm {
  period: string;
  start: string;
  end: string;
  category: string;
  search: string;
  repository: string;
  userId: string;
  kinds: string;
  excludeBots: boolean;
  rules: string;
}
