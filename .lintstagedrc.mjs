// Pre-commit formatting. Each language uses its own formatter and configuration:
//   web/          Prettier (web/.prettierrc with its plugins) and ESLint, run from web/
//   *.go          goimports (gofmt + import groups), same settings as core/.golangci.yml
//   *.rs          rustfmt (picks up the nearest rustfmt.toml)
//   other files   Prettier (root .prettierrc)
const quote = (files) => files.map((f) => JSON.stringify(f)).join(' ');

export default {
  'web/**/*.{ts,tsx,js,mjs,cjs}': (files) => [
    `pnpm --dir web exec eslint --fix ${quote(files)}`,
    `pnpm --dir web exec prettier --write ${quote(files)}`,
  ],
  'web/**/*.{json,css,md,yml,yaml}': (files) =>
    `pnpm --dir web exec prettier --write ${quote(files)}`,
  '**/*.go': (files) =>
    `go run golang.org/x/tools/cmd/goimports@v0.52.0 -local github.com/upayanmazumder/gitorgasm -w ${quote(files)}`,
  '**/*.rs': (files) => `rustfmt --edition 2024 ${quote(files)}`,
  '!(web)/**/*.{md,yml,yaml,json}': 'prettier --write --ignore-unknown',
  '*.{md,yml,yaml,json,mjs}': 'prettier --write --ignore-unknown',
};
