# Security Policy

## Report a vulnerability

Do not open a public issue for a security problem.
Use GitHub private vulnerability reporting on this repository (Security tab, "Report a vulnerability").
If that is not available, contact the maintainer, [@upayanmazumder](https://github.com/upayanmazumder), directly.

Include the affected component, the steps to reproduce, and the impact. You get an acknowledgement within 7 days.

## Scope

In scope: `core`, `apps/web`, `contracts`, `deploy`, and CI workflows.
Out of scope: GitHub itself, Vercel, Cloudflare, and the legacy GH Org Tool.

## Security model

The threat model is in [docs/architecture/security-threat-model.md](docs/architecture/security-threat-model.md). Main controls:

- Server-side tenant checks on every org-scoped request. Grants come from GitHub, not from client input.
- Opaque session ids. Only their SHA-256 hash is stored.
- GitHub user tokens are sealed with AES-256-GCM and bound to the user id.
- Webhooks need a valid HMAC-SHA256 signature. Each delivery is processed once.
- State-changing requests must come from the frontend origin.
- Containers run as non-root with a read-only root filesystem. NetworkPolicies deny traffic by default.

## Secrets

Never commit secrets or data exports. Production secrets live in the cluster repository, encrypted with SOPS and age.
