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

Open `http://localhost:3000/organizations`. Set these URLs in your development GitHub App:

- Callback URL: `http://localhost:3000/api/v1/auth/callback`.
- Setup URL: `http://localhost:3000/api/v1/auth/login`.
- Enable **Redirect on update**.
- Disable **Request user authorization (OAuth) during installation**. The app starts its own protected sign-in flow after installation.

Sign in, install the app for an organization, then complete sign-in when GitHub returns you to the website.
Use **Refresh GitHub access** after an organization owner approves installation or your access changes.

Run `make test` to check the project. See [CONTRIBUTING.md](CONTRIBUTING.md) for more commands.
