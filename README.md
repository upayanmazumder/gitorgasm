# Gitorgasm

Gitorgasm tracks GitHub organizations and contributor activity.
The platform collects organization data and displays contributor leaderboards on a web dashboard.

## Features

- Sign in with your GitHub account.
- View organizations and repositories in one place.
- Track contributor activity scores automatically.
- Receive updates through GitHub webhooks.

## Prerequisites

Install these tools before you start:

- Git
- Docker and Docker Compose
- Node.js (version 22 or later) and pnpm
- Go (version 1.24 or later)
- Rust

## Getting Started

Follow these steps to run Gitorgasm:

1. Clone the repository.

   ```bash
   git clone https://github.com/upayanmazumder/gitorgasm.git
   cd gitorgasm
   ```

2. Install development tools.

   ```bash
   make tools
   ```

3. Create your local environment file.

   ```bash
   make env
   ```

4. Start the database and API service.

   ```bash
   make up
   ```

5. Verify that the services respond.

   ```bash
   make smoke
   ```

The API service runs at `http://localhost:8080`.

## Useful Commands

| Command       | Action                                    |
| ------------- | ----------------------------------------- |
| `make up`     | Start the local database and API service. |
| `make down`   | Stop all running local services.          |
| `make test`   | Run the test suite.                       |
| `make lint`   | Check code format and style.              |
| `make verify` | Run all tests, checks, and builds.        |

## Documentation

- [Contributing Guide](CONTRIBUTING.md)
- [Agent Guide](AGENTS.md)
- [Security Policy](SECURITY.md)

## License

[MIT](LICENSE)
