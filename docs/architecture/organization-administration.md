# Organization administration

Implementation and review plan for issue 59. The public contract is
[`contracts/openapi/v1.yaml`](../../contracts/openapi/v1.yaml); the HTTP entry points
are `core/internal/httpapi/administration` and `httpapi/routes.go`.

## Preserve intent, not legacy implementation

The legacy organization tool has three useful workflows: invite multiple people,
add people to an existing team, and change existing members between Member and
Owner. GitHub's REST role `admin` means **organization Owner**, not a limited
application administrator. Owner grants full organization authority.

The old independent utility forms are not the interaction model to copy. Cached
browser permission checks, parallel invitation bursts, raw GitHub error messages,
and overwriting a team maintainer's role are unsafe. Request-bound synchronous
batches also lose their results when the browser disconnects. The old spreadsheet
parser is unnecessary dependency and input-surface weight.

The improved People workspace uses one selected-people list, action selection,
an explicit review step, and an itemized durable receipt. Scores remain available
through the existing organization navigation. Current members can be selected
from a paginated directory; public username search and pasted lists support
invitation and external-user selection. Public search can omit managed users, so
safe pasted usernames remain supported, including underscores.

## Existing architecture, no new service

This implements the bulk-operations use case already covered by accepted
[ADR 0005](../adr/0005-river-durable-jobs.md). The existing Go API, PostgreSQL and
River provide transactional enqueue and durable outcomes. There is no additional
broker, service, new ADR or browser-to-GitHub write path. The worker manifest gains
connection capacity through this pull request. No live resource is changed.

Responsibilities remain small and separate:

- `internal/administration`: canonical input and session-bound live authorization.
- `internal/github`: pinned REST API calls and typed minimal upstream models.
- `internal/store/administration`: SQL, idempotency, actor/tenant scoping and safe
  read-only interrupted projections.
- `internal/httpapi/administration`: HTTP validation, safe envelopes and atomic
  receipt/job/audit insertion.
- `internal/jobs/administration`: serial privileged execution and durable outcomes.
- The frontend People feature: bounded imports, selection, review, polling and
  recovery. It never receives GitHub credentials.

Two small durable tables store the operation and ordered targets. Operations
retain the actor, installation/account identity, originating session hash,
canonical fingerprint, idempotency key, timestamps and River job ID. They never
store plaintext tokens. There is no foreign key to `river_job`: application
migrations run before River migrations and completed jobs can be removed.

## Authorization and credential prerequisites

Every organization-scoped resource resolves the caller's current
`user_installations` grant before GitHub configuration, token or upstream checks.
Unknown and inaccessible organizations both return `404 organization_not_found`.

The shared authorizer opens only the sealed token associated with the exact
session cookie hash and internal user ID. It checks session/token expiry first.
The live installation is read with the App JWT and must match the granted stored
account identity, be an organization, not be suspended, and have Organization
Members **write**. The actor's GitHub App **user** token then checks live
`GET /user/memberships/orgs/{org}` and requires active `admin` membership.
Installation tokens are never substituted for actor authority. Capability reads
may return `can_manage=false` with a fixed reason; all other administration
resources require `can_manage=true`.

Deployment prerequisites requiring a human's separate approval are:

1. The GitHub App requests Organization Members write permission and the target
   organization approves that permission update.
2. An organization Owner signs in through the existing `/api/v1/auth/login` flow
   and has a current installation grant and usable user token.
3. The normal token encryption key, session infrastructure, PostgreSQL migrations,
   and River worker are configured through existing deployment mechanisms.

This feature documents these prerequisites; it does **not** modify a live App,
organization permission, installation, infrastructure or user role. Expired or
revoked GitHub tokens return `401 github_reauthentication_required`; no refresh
rotation is introduced. Session revocation or grant/owner/permission loss stops
worker execution at the next target's fresh authorization check.

## HTTP implementation plan and contract

All paths below are under `/v1/organizations/{login}` and require a session.
The existing global Origin middleware protects POST from cross-origin requests.
Malformed queries, bodies and cursors are rejected before GitHub authorization
calls. All errors use the existing Error envelope and fixed safe copy.

| Method and suffix | Result |
| --- | --- |
| GET `/administration` | Canonical organization, stable GitHub actor ID/login, live capability/reason, maximum 50 targets |
| GET `/administration/users?q=...&cursor=...` | One page of public user ID/login, next cursor and GitHub incomplete-results flag |
| GET `/administration/teams?cursor=...` | One page of team ID/name/slug only |
| GET `/administration/members?role=all\|member\|admin&cursor=...` | One page of current member ID/login; no fabricated per-row role |
| POST `/administration/operations` | 202 active receipt, or 200 terminal idempotent replay, plus relative Location |
| GET `/administration/operations/{id}` | Full original-actor receipt under current owner/grant authorization |
| GET `/administration/operations?cursor=...` | Latest ten original-actor summaries, strict created-time/ID keyset pagination |

Submission accepts at most 16 KiB of JSON, one UUID `Idempotency-Key`, no unknown
fields or trailing JSON, and only the fields appropriate to the action. Logins
are trimmed, one optional leading `@` is removed, then safe segments matching
`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$` are deduplicated case-insensitively in stable
input order. The maximum is 50 unique users. Invite/team actions are Member-only;
role changes require explicit `member` or `admin`. A team must already exist.

The operation and targets are inserted in the same transaction as
`queue.AdministerOrganization{OperationID}`, the stored River job ID and audit
`organization.administration_requested`. Audit metadata contains action kind,
target count and installation identity, not usernames or credentials. The job
contains only the opaque operation ID. The same actor/installation/key and
canonical payload returns the same receipt without another job or audit;
a changed payload returns 409. Terminal replay uses the same interrupted-view
logic as receipt reads. Team validation happens only for a newly created request,
so later team removal does not invalidate a historical idempotent replay.

User search accepts 2–64 ASCII letters, digits, underscores, hyphens and spaces,
normalizes case/spaces, and appends a fixed `type:user` restriction. GitHub public
user search receives **no Authorization header**. Directory requests fetch one
30-item page, not every page. Opaque base64url cursors bind actor, organization,
resource and normalized query/role; search pages are bounded to 34 (GitHub's
1,000-result window), other directory pages to 10,000. They never carry an
arbitrary upstream URL. Receipt-history cursors carry only the scoped
keyset position; changing a cursor cannot grant another actor's rows. Private
team metadata and user emails are not included in responses.

## Execution and durable failure behavior

The dedicated administration queue has one worker per process, `MaxAttempts: 1`
and a 15-minute execution timeout. A global PostgreSQL advisory lock serializes
batches across replicas. Writes are serial with at least one second before each
target, including the first target of a new batch; this preserves pacing when a
different replica takes the next batch. After spacing and before each target,
revalidate the exact session, grant, token, installation permissions and live
Owner status. Workers claim a batch once and persist a target as running
**before** its external write. Restart/duplicate jobs must not replay writes.
The worker pool minimum is `2 * WORKER_CONCURRENCY + 7` connections (15 at default
concurrency 4), including the extra administration worker and lock headroom.

- Invitations resolve a stable user ID and leave active members and existing
  pending invitations unchanged; they do not resend notifications automatically.
- Team assignment leaves existing active/pending membership, including
  maintainers, unchanged. A new membership is Member-only and can invite someone
  who is not yet in the organization.
- Organization role changes require an existing active member. Self-demotion is
  forbidden by stable GitHub user ID, regardless of login casing or rename.
  GitHub's role endpoint can invite a nonmember. A concurrent removal between the
  membership check and the write cannot be excluded atomically. If GitHub returns
  a pending or unexpected state, the item becomes `unknown/membership_changed`.
  Stop the remaining batch. The review warns about this upstream race.
- A definite target-specific rejection can fail that target and continue.
  Authentication, global permission loss, rate limits and outages stop the
  remaining batch, preserving earlier outcomes.
- A timeout, disconnect or server error after a write may mean GitHub applied it.
  Mark that target `unknown/outcome_unknown`, never automatically retry it, and
  instruct the user to check GitHub before starting another operation.
- Graceful cancellation finalizes unknown/not-attempted outcomes. If a process is
  killed, receipt reads derive an `interrupted` terminal view from missing or
  terminal River jobs or attempts older than 30 minutes. Running targets become
  unknown, queued targets become not_attempted, and completed results remain.
  The derived receipt has a completion timestamp; GET never repairs rows.

Messages come from a fixed code dictionary, never GitHub response bodies. Receipt
results distinguish succeeded, invited, unchanged, failed, unknown and
not_attempted; pending invitations are not represented as completed membership.

## Frontend implementation plan

Use existing Sunbeam components, typography, tokens, accessible navigation and
TanStack Query. Provide paste, CSV and JSON imports, not binary Excel. Bound
UTF-8 imports to 64 KiB and selection to 50 users. Recognize a documented
login/username/github/github_username column or a single-column CSV list, handle
quoted CSV and a BOM, and show every invalid nonblank row rather than silently
skipping it. Imports deduplicate visibly with the same username semantics.

Review shows the exact organization, every target, team and requested role.
Warn that team adds can invite nonmembers and Owner grants full organization
control. Input edits invalidate review. No mutation occurs before confirmation.
Keep submitted intent and idempotency key stable after an uncertain enqueue
response; do not silently retry mutations. Disable self-demotion in the interface
using the capability actor's stable ID, independently of the server guard.

Persist the active receipt ID in the URL and show recent actor-only receipts for
lost-response recovery. Query keys include organization/receipt identity; poll
only queued/running receipts, stop on terminal results, unmount and hidden tabs,
and use explicit cancellation. Expose accessible progress/live status, visible
keyboard focus, semantic labels, 44px targets and keyboard-operable review.

## Integration acceptance plan

Preserved HTTP behavior tests cover tenant/session/live-owner gates, per-session
credentials, strict requests, CSRF, idempotent receipts, actor-only history,
interrupted read-only receipts, safe projection and directory cursor confinement.
Additional tests cover invalid input rejection before any GitHub calls and
transactional audit failure rollback/no duplicate audits. Real-response OpenAPI
validation remains part of the integration harness.

The integration owner runs formatting, Go/Rust/frontend gates, database
fresh/upgrade migration tests, and an isolated API/worker/PostgreSQL smoke against
a local GitHub HTTP fixture. Security and frontend review must cover revoked
sessions, owner/self-demotion boundaries, write ambiguity, no credential leakage,
responsive light/dark layouts and keyboard completion. No acceptance test sends
real invitations or changes live GitHub permissions.

## Official GitHub references

REST API version remains `2022-11-28` as pinned in the existing client.

- [Organization membership, listing and invitations](https://docs.github.com/en/rest/orgs/members)
- [Team membership endpoints](https://docs.github.com/en/rest/teams/members)
- [Team listing](https://docs.github.com/en/rest/teams/teams)
- [User identity lookup](https://docs.github.com/en/rest/users/users)
- [Public user search and authentication requirements](https://docs.github.com/en/rest/search/search#search-users)
- [GitHub App REST permission requirements](https://docs.github.com/en/rest/authentication/permissions-required-for-github-apps)
- [Organization roles and Owner authority](https://docs.github.com/en/organizations/managing-peoples-access-to-your-organization-with-roles/roles-in-an-organization)
