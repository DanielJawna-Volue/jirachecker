# jirachecker

Finds feature branches in a GitHub repository whose Jira issues are closed, so they can be deleted.

A branch is a candidate when its name looks like `prefix/rest`, where `prefix` is one of
`feat, feature, bug, bugfix, fix, hotfix, chore, task` (case-insensitive, configurable with
`-prefixes`) and `rest` contains a Jira key such as `ABC-123` or `abc-123`. A branch is
**deletable** when every Jira key in its name is in the *Done* status category (Done, Closed,
Resolved, …). The default branch and protected branches are never listed.

## Setup

```sh
gh auth login                       # GitHub token is read via `gh auth token`
export JIRA_URL=https://yourcompany.atlassian.net
export JIRA_API_TOKEN=...           # Cloud: API token; Server/DC: personal access token
export JIRA_EMAIL=you@company.com   # Cloud only
go build -o jirachecker .
```

## Usage

```sh
jirachecker list owner/repo          # deletable branches
jirachecker list owner/repo -all     # also show kept branches and why
```

Deleting branches is not implemented yet.
