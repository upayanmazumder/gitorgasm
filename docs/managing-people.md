# Manage organization people

Organization owners can open **People** from an organization's score page.
The GitHub App installation must have approved Organization Members **write** permission.

- Invite people, add people to an existing team, or change Member/Organization owner roles.
- Select current members or public search results. Paste usernames or import CSV/JSON files up to 64 KiB.
- Review up to 50 unique usernames, the organization, and the requested team or role before confirming.
- Follow the durable receipt and its individual results. Closing the tab does not cancel a queued request.

Existing team maintainers retain their role. Role changes reject known nonmembers and block self-demotion.
GitHub can invite someone removed concurrently during a role change. An unexpected membership state stops the batch with an uncertain result.
Check uncertain results on GitHub before submitting another request. Sign-out, expired access, or lost ownership stops remaining changes.

See the [administration plan and security model](architecture/organization-administration.md) for the API, permission requirements, and failure behavior.
