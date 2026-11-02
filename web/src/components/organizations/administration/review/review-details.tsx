import { StatusMessage } from '@/components/ui';

import {
  ACTION_LABELS,
  describedFrozen,
  describeRequest,
  personNote,
  ROLE_LABELS,
  type Frozen,
} from '../draft';

interface Props {
  org: string;
  request: Frozen;
}

function Notes({ org, request }: Props) {
  const { body } = request;
  const unconfirmed = request.unconfirmed.length;
  if (body.kind === 'invite') {
    return (
      <p>
        GitHub notifies each person. People who are already members or already invited are not
        changed, and Gitorgasm does not send them another invitation.
      </p>
    );
  }
  if (body.kind === 'team_membership') {
    return (
      <>
        <p>
          Team maintainers keep their maintainer role. People already on the team are not changed.
        </p>
        {unconfirmed > 0 && (
          <p>
            {unconfirmed === 1 ? '1 person is' : `${unconfirmed} people are`} not confirmed as
            members of {org}. GitHub may send them an invitation to the organization. They join the
            team after they accept it.
          </p>
        )}
      </>
    );
  }
  return (
    <>
      {body.role === 'admin' ? (
        <StatusMessage role="status" tone="error">
          Organization owners get complete administrative access to {org}. Make only people you
          trust owners.
        </StatusMessage>
      ) : (
        <p>
          Owners in this request lose owner access to {org}. People who are not active members are
          not changed.
        </p>
      )}
      <p>
        If someone leaves the organization while this request runs, GitHub may send them an
        invitation. Check any uncertain result on GitHub.
      </p>
    </>
  );
}

export function ReviewDetails({ org, request }: Props) {
  const { body, teamName } = request;
  return (
    <div className="space-y-6">
      <p className="font-semibold">{describeRequest(describedFrozen(request), org)}</p>
      <dl className="grid gap-x-6 gap-y-2 md:grid-cols-[max-content_1fr]">
        <dt className="text-sm font-medium text-muted-foreground">Organization</dt>
        <dd className="font-semibold wrap-anywhere">{org}</dd>
        <dt className="text-sm font-medium text-muted-foreground">Action</dt>
        <dd className="font-semibold">{ACTION_LABELS[body.kind]}</dd>
        {body.kind === 'team_membership' && (
          <>
            <dt className="text-sm font-medium text-muted-foreground">Team</dt>
            <dd className="font-semibold wrap-anywhere">{teamName}</dd>
          </>
        )}
        {body.kind === 'organization_role' && (
          <>
            <dt className="text-sm font-medium text-muted-foreground">New role</dt>
            <dd className="font-semibold">{ROLE_LABELS[body.role]}</dd>
          </>
        )}
        <dt className="text-sm font-medium text-muted-foreground">People</dt>
        <dd className="font-semibold tabular-nums">{body.logins.length}</dd>
      </dl>
      <div className="max-w-[60ch] space-y-2">
        <Notes org={org} request={request} />
      </div>
      <div className="space-y-2">
        <h3 id="people-review-list-heading">People in this request</h3>
        <ol
          aria-labelledby="people-review-list-heading"
          className="list-decimal gap-8 pl-6 md:columns-2 lg:columns-3"
        >
          {body.logins.map(login => {
            const unconfirmed =
              body.kind === 'team_membership' && request.unconfirmed.includes(login);
            const person = { login, key: login.toLowerCase(), id: null, member: false };
            return (
              <li key={login} className="break-inside-avoid wrap-anywhere">
                {login}
                {unconfirmed && (
                  <span className="block text-sm text-muted-foreground">
                    {personNote(person, body.kind, org)}
                  </span>
                )}
              </li>
            );
          })}
        </ol>
      </div>
    </div>
  );
}
