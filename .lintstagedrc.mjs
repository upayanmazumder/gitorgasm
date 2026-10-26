// Run Go formatting per module so golangci-lint reads the matching config.
const quote = (files) => files.map((f) => JSON.stringify(f)).join(' ');
const golangciFmt = (files) =>
  `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 fmt ${quote(files)}`;

export default {
  'web/**/*.{ts,tsx,js,mjs,cjs}': (files) => [
    `pnpm --dir web exec eslint --fix ${quote(files)}`,
    `pnpm --dir web exec prettier --write ${quote(files)}`,
  ],
  'web/**/*.{json,css,md,yml,yaml}': (files) =>
    `pnpm --dir web exec prettier --write ${quote(files)}`,
  'core/**/*.go': golangciFmt,
  'tests/performance/lang-bench/go/**/*.go': golangciFmt,
  '**/*.rs': (files) => `rustfmt ${quote(files)}`,
};
