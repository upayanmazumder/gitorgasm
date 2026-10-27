# Gitorgasm

Gitorgasm shows activity and scores for the GitHub organizations you can access. You can choose a time period, search, filter results, and save your choices.

## Start locally

Install Docker, Node.js, and pnpm. Then run:

```bash
git clone https://github.com/upayanmazumder/gitorgasm.git
cd gitorgasm
make env
make up
make smoke
```

This starts the database and app server. The smoke check works without a GitHub account.

To see your organizations, create a development GitHub App. Give it read access to repository contents, pull requests, and issues. Add its details to `.env.local`, then run `make up-github`.

Start the website in another terminal:

```bash
cd web
printf 'API_ORIGIN=http://localhost:8080\n' > .env.local
pnpm install
pnpm dev
```

Open `http://localhost:3000`. For GitHub sign-in, set the App callback to `http://localhost:3000/api/v1/auth/callback`.

Run `make test` to check the project. See [CONTRIBUTING.md](CONTRIBUTING.md) for more commands.
