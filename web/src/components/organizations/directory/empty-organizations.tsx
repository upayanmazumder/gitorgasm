export function EmptyOrganizations() {
  return (
    <div className="space-y-2 rounded-lg border border-dashed p-6">
      <h3 className="font-medium">No organizations available yet</h3>
      <p className="text-sm text-muted-foreground">
        Install the GitHub App for an organization, then complete the GitHub sign-in when you
        return. If installation needs approval, ask an organization owner to approve it and refresh
        GitHub access afterward.
      </p>
    </div>
  );
}
