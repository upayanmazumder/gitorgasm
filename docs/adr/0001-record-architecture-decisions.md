# ADR 0001: Record architecture decisions

- Status: Accepted
- Date: 2026-10-26

## Context

Gitorgasm replaces the legacy GH Org Tool. The project has one maintainer (risk R8 in `docs/architecture/risk-register.md`). Decisions made during the bootstrap must stay traceable after the original context is gone.

## Decision

Record each significant architecture decision as a Markdown file in `docs/adr/` named `NNNN-title.md`. Each record has Status, Date, Context, Decision, Consequences, and Alternatives. Write records in Simplified Technical English. Label facts as **Verified** (with a source), **Assumption**, or **Planned**. A record is immutable after acceptance. To change a decision, add a new record and set the old status to `Superseded by NNNN`. List every record in `docs/adr/README.md`.

## Consequences

- Reviewers can find why a choice was made without chat history.
- Every new decision costs a short document.
- Superseded records stay in the repository as history.

## Alternatives

- Decisions only in `docs/architecture/*.md`: rejected. Those documents change in place and lose history.
- Wiki or issue tracker: rejected. Not versioned with the code.
