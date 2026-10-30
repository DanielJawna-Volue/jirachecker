# jirachecker

Finds feature branches in a GitHub repository whose Jira issues are closed, so they can be deleted.

A branch is a candidate when its name looks like `prefix/rest`, where `prefix` is one of
`feat, feature, bug, bugfix, fix, hotfix, chore, task` (case-insensitive, configurable with
`-prefixes`) and `rest` contains a Jira key such as `ABC-123` or `abc-123`. A branch is
**deletable** when every Jira key in its name is in the *Done* status category (Done, Closed,
Resolved, …). The default branch and protected branches are never listed.

## Build

Prerequisites:

- [Go](https://go.dev/dl/) 1.23 or newer (`go version` to check). On Windows: `winget install --id GoLang.Go -e`
- [GitHub CLI](https://cli.github.com/) (`gh`), needed at runtime for the GitHub token

The project uses only the Go standard library, so there is nothing to download besides Go itself.

Bash / zsh:

```sh
git clone <repo-url> jirachecker
cd jirachecker
go test ./...                 # run the unit tests
go build -o jirachecker .     # produces ./jirachecker
```

PowerShell:

```powershell
git clone <repo-url> jirachecker
cd jirachecker
go test ./...                    # run the unit tests
go build -o jirachecker.exe .    # produces .\jirachecker.exe
```

To install it onto your PATH instead (into `$(go env GOPATH)/bin`), run `go install .` from the
repository root.

### Building for another OS

Set `GOOS` and `GOARCH` to cross-compile, e.g. a Linux binary from any machine:

```sh
GOOS=linux GOARCH=amd64 go build -o jirachecker-linux-amd64 .
```

```powershell
$env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -o jirachecker-linux-amd64 .
Remove-Item Env:GOOS, Env:GOARCH   # reset, otherwise later builds also target Linux
```

Common targets: `windows/amd64`, `linux/amd64`, `darwin/arm64` (Apple Silicon), `darwin/amd64`.

## Setup

The tool needs access to GitHub and to Jira.

**GitHub:** no token to set. The tool runs `gh auth token`, so log in once with `gh auth login`
(for GitHub Enterprise, log in to that host and pass `-github-host your.host`).

**Jira:** configured with environment variables. Which ones you need depends on your Jira:

| Variable         | Jira Cloud (`*.atlassian.net`)                                                                | Jira Server / Data Center                |
|------------------|-----------------------------------------------------------------------------------------------|------------------------------------------|
| `JIRA_URL`       | required                                                                                      | required                                 |
| `JIRA_API_TOKEN` | required: an API token without scopes ([how](#creating-the-jira-token))                       | required: a personal access token ([how](#creating-the-jira-token)) |
| `JIRA_EMAIL`     | required: the email of the account that owns the token                                        | **must not be set**                      |

`JIRA_EMAIL` decides how the token is sent: if it is set, the tool uses basic auth
(email + API token, as Jira Cloud expects); if it is unset, it sends the token as a bearer
token (as Server/DC personal access tokens expect). `JIRA_URL` and `JIRA_EMAIL` can also be
given as the `-jira-url` / `-jira-email` flags; the token can only come from the environment.

### Creating the Jira token

**Jira Cloud:** on the [API tokens page](https://id.atlassian.com/manage-profile/security/api-tokens)
choose **Create API token** (not *Create API token with scopes*). Give it a name and an expiry,
then copy the token; it is shown only once. It has the same permissions as your account, and the
tool only reads your profile and issue statuses.

Scoped tokens are not supported by the tool: Atlassian accepts them only through the
`https://api.atlassian.com/ex/jira/<cloud-id>` gateway, not through your site URL
(`https://yourcompany.atlassian.net`) that `JIRA_URL` points to.

**Jira Server / Data Center:** personal access tokens are not created on the Atlassian page above
but in Jira itself: click your avatar → **Profile** → **Personal Access Tokens** →
**Create token**.

Bash / zsh:

```sh
gh auth login
export JIRA_URL=https://yourcompany.atlassian.net
export JIRA_API_TOKEN=...
export JIRA_EMAIL=you@company.com   # omit for Server/DC
```

PowerShell:

```powershell
gh auth login
$env:JIRA_URL = "https://yourcompany.atlassian.net"
$env:JIRA_API_TOKEN = "..."
$env:JIRA_EMAIL = "you@company.com"   # omit for Server/DC
```

`$env:` variables last for the current session only. To keep them for your user account, use
`[Environment]::SetEnvironmentVariable("JIRA_URL", "https://yourcompany.atlassian.net", "User")`
(and likewise for the others), then open a new terminal.

## Usage

Bash / zsh:

```sh
./jirachecker list owner/repo                      # deletable branches
./jirachecker list owner/repo -all                 # also show kept branches and why
./jirachecker multi owner/repo1 owner/repo2        # several repositories
./jirachecker multi owner/repo1,owner/repo2 -all   # commas work too
```

PowerShell:

```powershell
.\jirachecker.exe list owner/repo                      # deletable branches
.\jirachecker.exe list owner/repo -all                 # also show kept branches and why
.\jirachecker.exe multi owner/repo1 owner/repo2        # several repositories
.\jirachecker.exe multi owner/repo1,owner/repo2 -all   # commas work too
```

`multi` prints the same report as `list` for each repository, then a summary table with the
number of deletable branches per repository. If one repository fails (for example it does not
exist or you have no access), the error is shown in the summary, the other repositories are still
scanned, and the exit code is 1. The flags (`-all`, `-prefixes`, …) apply to every repository.
Run `jirachecker list -h` or `jirachecker multi -h` for all flags.

Deleting branches is not implemented yet.
