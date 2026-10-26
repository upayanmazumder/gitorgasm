// Generates golden fixtures for the Rust scoring crate by running the LEGACY GH Org Tool
// TypeScript engine (lib/scoring) on deterministic synthetic signals.
//
// The legacy repository is read only. Bun must run with the legacy repository as the working
// directory so that its `@/` path alias resolves:
//
//   cd <legacy repo> && bun <gitorgasm>/scoring/tests/golden/generate.ts <gitorgasm>/scoring/tests/golden
//
// Inputs are synthetic (logins user-NN, repos repo-N). No production data is used.
import { writeFileSync } from 'node:fs';
import { join } from 'node:path';

// Dynamic imports: the legacy checkout location is chosen at run time (the working directory),
// so static import specifiers cannot name it.
const legacy = process.cwd();
const engine = await import(join(legacy, 'lib/scoring/engine.ts'));
const aggregate = await import(join(legacy, 'lib/scoring/aggregate.ts'));
const rules = await import(join(legacy, 'lib/scoring/rules.ts'));

const outDir = process.argv[2];
if (!outDir) throw new Error('usage: generate.ts <output dir>');

// Deterministic PRNG (mulberry32) so fixtures are reproducible.
function rng(seed: number) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const TYPES = [
  'commit',
  'pr_open',
  'pr_merge',
  'pr_close_no_merge',
  'issue_open',
  'issue_close',
  'review',
  'review_comment',
  'comment',
  'spam',
] as const;
const TYPE_WEIGHTS = [40, 6, 5, 2, 4, 4, 10, 8, 12, 1];
const STATES = ['approved', 'changes_requested', 'commented', 'dismissed'];
const OFFSETS = ['Z', 'Z', '+00:00', '+05:30', '-07:00'];

function pick<T>(r: () => number, xs: readonly T[], weights?: number[]): T {
  if (!weights) return xs[Math.floor(r() * xs.length)];
  const total = weights.reduce((a, b) => a + b, 0);
  let x = r() * total;
  for (let i = 0; i < xs.length; i++) {
    x -= weights[i];
    if (x < 0) return xs[i];
  }
  return xs[xs.length - 1];
}

const pad = (n: number, w = 2) => String(n).padStart(w, '0');

// Format an instant in a given UTC offset, as GitHub does for GitTimestamp values.
function formatTs(ms: number, offset: string): string {
  if (offset === 'Z' || offset === '+00:00') {
    return new Date(ms).toISOString().replace('.000Z', offset === 'Z' ? 'Z' : '+00:00');
  }
  const sign = offset[0] === '-' ? -1 : 1;
  const mins = sign * (Number(offset.slice(1, 3)) * 60 + Number(offset.slice(4, 6)));
  const d = new Date(ms + mins * 60_000);
  return (
    `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}` +
    `T${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}${offset}`
  );
}

function makeSignals(seed: number, count: number, users: number, repos: number) {
  const r = rng(seed);
  const start = Date.UTC(2026, 0, 3, 18, 0, 0); // a Saturday evening: crosses a week boundary early
  const signals = [];
  for (let i = 0; i < count; i++) {
    const userId = 1 + Math.floor(r() * r() * users); // skewed: a few heavy contributors
    const type = pick(r, TYPES, TYPE_WEIGHTS);
    // Bursts: many events land on the same few days to hit quotas and diminishing returns.
    const day = Math.floor(r() * r() * 70);
    const ms = start + day * 86_400_000 + Math.floor(r() * 86_400) * 1000;
    // Some events share an exact timestamp to exercise stable sorting.
    const tsMs =
      r() < 0.05 && signals.length > 0
        ? Date.parse(signals[signals.length - 1].eventTimestamp)
        : ms;
    const metadata: Record<string, unknown> = {};
    if (r() < 0.06) metadata.isBot = true;
    else if (r() < 0.5) metadata.isBot = false;
    if (type === 'commit') {
      metadata.additions = Math.floor(r() * 400);
      metadata.deletions = Math.floor(r() * 200);
      if (r() < 0.5) metadata.isInMergedPR = r() < 0.6;
      if (r() < 0.1) metadata.hasLinkedIssue = true;
    }
    if (type === 'pr_merge') {
      metadata.isSelfMerge = r() < 0.3;
      if (r() < 0.6) metadata.hasLinkedIssue = r() < 0.5;
    }
    if (type === 'review') {
      if (r() < 0.9) metadata.state = pick(r, STATES);
      metadata.isSelfReview = r() < 0.1;
    }
    if (type === 'issue_close' && r() < 0.7) metadata.hasLinkedPR = r() < 0.5;
    signals.push({
      userId,
      userLogin: `user-${pad(userId)}`,
      type,
      value: 1,
      repo: `repo-${1 + Math.floor(r() * repos)}`,
      eventTimestamp: formatTs(tsMs, pick(r, OFFSETS)),
      contentHash: `h${seed}-${i}`,
      metadata,
    });
  }
  return signals;
}

function randomRuleset(seed: number) {
  const r = rng(seed);
  const base = rules.defaultScoringRuleset();
  const scorable = TYPES.filter((t) => t !== 'spam');
  return {
    basePoints: scorable
      .filter(() => r() < 0.85)
      .map((type) => ({ type, points: Math.round(r() * 60 * 4) / 4 })),
    multipliers: [
      {
        kind: 'first_activity',
        factor: 1 + Math.round(r() * 100) / 100,
        appliesTo: scorable.filter(() => r() < 0.6),
      },
      {
        kind: 'merged_pr_commit',
        factor: 1 + Math.round(r() * 50) / 100,
        appliesTo: ['commit', 'review'],
      },
      {
        kind: 'pr_linked_to_issue',
        factor: 1 + Math.round(r() * 50) / 100,
        appliesTo: ['pr_merge', 'commit'],
      },
      // A duplicate kind: legacy uses the first match only.
      { kind: 'first_activity', factor: 9, appliesTo: scorable },
    ],
    reviewStateWeights: [
      { state: 'approved', weight: Math.round(r() * 200) / 100 },
      { state: 'changes_requested', weight: Math.round(r() * 200) / 100 },
    ],
    dailyQuotas: scorable
      .filter(() => r() < 0.4)
      .map((type) => ({ type, limit: 1 + Math.floor(r() * 5) })),
    diminishingReturns: {
      weeklyThreshold: Math.floor(r() * 12),
      decayFactor: Math.round(r() * 30) / 100,
      floorFraction: Math.round(r() * 50) / 100,
    },
    zeroPointConditions: base.zeroPointConditions.map((c: { kind: string }) => ({
      kind: c.kind,
      enabled: r() < 0.5,
    })),
    spamPenalty: -Math.floor(r() * 20),
    prClosedNoMergePenalty: -Math.floor(r() * 15),
  };
}

function memberships(seed: number, users: number) {
  const r = rng(seed);
  const out = [];
  for (let t = 1; t <= 4; t++) {
    const scoped = t % 2 === 0;
    const repoNames = scoped ? [`repo-${t}`, `repo-${t + 1}`] : undefined;
    for (let u = 1; u <= users; u++) {
      if (r() < 0.35) {
        out.push({
          userId: u,
          userLogin: `user-${pad(u)}`,
          teamSlug: `team-${t}`,
          teamName: `Team ${t}`,
          teamId: t,
          ...(repoNames ? { repositoryNames: repoNames } : {}),
        });
      }
    }
  }
  return out;
}

function run(name: string, signals: unknown[], ruleset: unknown, teams: unknown[]) {
  const scores = engine.computeScores(signals, ruleset);
  const fixture = {
    name,
    generator: 'scoring/tests/golden/generate.ts',
    ruleset,
    signals,
    teamMemberships: teams,
    expected: {
      contributors: engine.buildLeaderboard(scores),
      repositories: aggregate.aggregateByRepository(signals, ruleset),
      teams: aggregate.aggregateByTeamSignals(signals, ruleset, teams),
    },
  };
  writeFileSync(join(outDir, `${name}.json`), JSON.stringify(fixture) + '\n');
  console.log(
    `${name}: ${signals.length} signals, ${fixture.expected.contributors.length} contributors`,
  );
}

run(
  'default-ruleset',
  makeSignals(1, 2500, 30, 6),
  rules.defaultScoringRuleset(),
  memberships(11, 30),
);
for (let i = 1; i <= 4; i++) {
  run(
    `random-ruleset-${i}`,
    makeSignals(100 + i, 1200, 20, 5),
    randomRuleset(200 + i),
    memberships(300 + i, 20),
  );
}
run('empty', [], rules.defaultScoringRuleset(), []);
